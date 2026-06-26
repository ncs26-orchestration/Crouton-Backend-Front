"""Attachment text extraction.

The Go API is authoritative for attachment persistence: it stores the
row in `chat_attachments` with the `text_content` we return here. This
endpoint just does the file -> text conversion so the extractor sees
a normalized string. We intentionally do NOT keep the binary.

v0.1 scope (all local — no cloud calls):
  - PDF:    parsed with `pypdf` (pure-Python).
  - TXT:    UTF-8 / latin-1 passthrough.
  - Voice:  faster-whisper running in-process, model cached in
            $WHISPER_CACHE_DIR. ffmpeg in the image decodes m4a/mp3/etc.
  - Image:  pytesseract over the system tesseract binary; French + English
            language packs preinstalled.
"""

from __future__ import annotations

import io
import logging
import os
import tempfile
from typing import Literal

from fastapi import APIRouter, Form, HTTPException, UploadFile
from pydantic import BaseModel

from app.settings import settings

log = logging.getLogger(__name__)

router = APIRouter(prefix="/attachments", tags=["attachments"])

# Keep the per-file cap well under the Go handler's limit so a large
# upload fails fast at the boundary with a useful message.
_MAX_BYTES = 8 * 1024 * 1024


AttachmentKind = Literal["document", "voice", "image"]


class ExtractTextResponse(BaseModel):
    kind: AttachmentKind
    filename: str
    mime: str
    size_bytes: int
    text_content: str
    # The Go side uses the first ~1500 chars in the extractor prompt
    # and stores the full text_content in Postgres. We don't truncate
    # here — the DB row is the archival copy.


def _kind_for(mime: str, filename: str) -> AttachmentKind:
    m = (mime or "").lower()
    f = filename.lower()
    if m.startswith("audio/") or f.endswith((".mp3", ".wav", ".m4a", ".webm", ".ogg")):
        return "voice"
    if m.startswith("image/") or f.endswith((".png", ".jpg", ".jpeg", ".gif", ".webp")):
        return "image"
    return "document"


def _extract_pdf(data: bytes) -> str:
    # Imported lazily so unit tests that don't exercise PDF paths
    # don't pay the import cost.
    from pypdf import PdfReader

    reader = PdfReader(io.BytesIO(data))
    chunks: list[str] = []
    for page in reader.pages:
        try:
            text = page.extract_text() or ""
        except Exception:  # noqa: BLE001 — malformed page, skip but keep going
            text = ""
        if text.strip():
            chunks.append(text)
    return "\n\n".join(chunks).strip()


def _extract_text(data: bytes) -> str:
    # Try UTF-8 first; fall back to latin-1 so legacy Windows exports
    # don't crash the upload. Latin-1 is always decodable because every
    # byte maps to a codepoint.
    try:
        return data.decode("utf-8").strip()
    except UnicodeDecodeError:
        return data.decode("latin-1", errors="replace").strip()


# Whisper model is loaded lazily on first use and cached on the module
# for the lifetime of the process. faster-whisper is happy to be reused
# concurrently — its internal threadpool serializes when needed.
_whisper_model = None  # type: ignore[var-annotated]


def _get_whisper_model() -> object:
    global _whisper_model
    if _whisper_model is None:
        from faster_whisper import WhisperModel  # type: ignore[import-untyped]

        os.makedirs(settings.whisper_cache_dir, exist_ok=True)
        log.info(
            "loading whisper model %s (compute=%s, cache=%s)",
            settings.whisper_model,
            settings.whisper_compute_type,
            settings.whisper_cache_dir,
        )
        _whisper_model = WhisperModel(
            settings.whisper_model,
            device="cpu",
            compute_type=settings.whisper_compute_type,
            download_root=settings.whisper_cache_dir,
        )
    return _whisper_model


def _transcribe_voice(data: bytes, suffix: str) -> str:
    """Run faster-whisper on a voice clip. Auto-detects language so
    French / Arabic / English clips all work without a hint.

    The model's first call is slow (model download on cold cache);
    warm calls are realtime-or-better on small clips. Returns the
    full transcript with whitespace normalized.
    """
    model = _get_whisper_model()
    # faster-whisper takes a path, so we materialize the upload to a
    # tempfile. ffmpeg in the image handles every common container.
    with tempfile.NamedTemporaryFile(suffix=suffix or ".bin", delete=True) as tmp:
        tmp.write(data)
        tmp.flush()
        segments, info = model.transcribe(  # type: ignore[attr-defined]
            tmp.name,
            beam_size=1,  # greedy — faster, fine for short interview clips
            vad_filter=True,  # skip silence; cuts cost on noisy recordings
        )
        chunks = [seg.text.strip() for seg in segments if seg.text]
    text = " ".join(c for c in chunks if c).strip()
    if info and getattr(info, "language", None):
        log.info("whisper transcribed %d chars (lang=%s)", len(text), info.language)
    return text


def _ocr_image(data: bytes) -> str:
    """Tesseract OCR over a PNG/JPG. Tries French+English jointly so
    procedure screenshots in either language come out clean. Returns
    whitespace-normalized text.
    """
    import pytesseract
    from PIL import Image

    img = Image.open(io.BytesIO(data))
    # Multi-language: tesseract accepts "fra+eng" as a language spec.
    # The fra and eng packs are installed in the Dockerfile.
    text = pytesseract.image_to_string(img, lang="fra+eng")
    return " ".join(text.split())


@router.post("/extract-text", response_model=ExtractTextResponse)
async def extract_text(
    file: UploadFile,
    # Optional overrides — the Go side can pass an authoritative kind
    # when it recognizes the filename better than sniffing does.
    kind_hint: str | None = Form(default=None),
) -> ExtractTextResponse:
    data = await file.read()
    if not data:
        raise HTTPException(status_code=400, detail="empty_upload")
    if len(data) > _MAX_BYTES:
        raise HTTPException(status_code=413, detail=f"file_too_large_{_MAX_BYTES}_bytes")

    filename = file.filename or "upload"
    mime = file.content_type or "application/octet-stream"
    kind = kind_hint if kind_hint in ("document", "voice", "image") else _kind_for(mime, filename)

    # Route to the extractor matching the detected kind.
    text = ""
    lower = filename.lower()
    if kind == "document":
        if lower.endswith(".pdf") or mime == "application/pdf":
            try:
                text = _extract_pdf(data)
            except Exception as exc:  # noqa: BLE001
                raise HTTPException(status_code=422, detail=f"pdf_parse_failed: {exc}") from exc
        elif lower.endswith((".txt", ".md", ".csv", ".log")) or mime.startswith("text/"):
            text = _extract_text(data)
        else:
            # DOCX and other "document"-ish types — stub until we wire
            # a real parser. Chip still shows up in the UI with this
            # placeholder, so the flow is visible end-to-end.
            text = f"[unsupported document type: {mime or 'unknown'} — transcription pending]"
    elif kind == "voice":
        # Pick a tempfile suffix from the original filename so ffmpeg
        # has the format hint. faster-whisper / ffmpeg handle mp3,
        # m4a, webm, wav, ogg out of the box.
        suffix = ""
        for ext in (".mp3", ".m4a", ".webm", ".wav", ".ogg"):
            if lower.endswith(ext):
                suffix = ext
                break
        try:
            text = _transcribe_voice(data, suffix)
            if not text:
                text = "[silence — no speech detected in clip]"
        except Exception as exc:  # noqa: BLE001
            log.exception("whisper transcription failed")
            raise HTTPException(status_code=422, detail=f"asr_failed: {exc}") from exc
    elif kind == "image":
        try:
            text = _ocr_image(data)
            if not text:
                text = "[no text detected in image]"
        except Exception as exc:  # noqa: BLE001
            log.exception("tesseract ocr failed")
            raise HTTPException(status_code=422, detail=f"ocr_failed: {exc}") from exc

    return ExtractTextResponse(
        kind=kind,  # type: ignore[arg-type]
        filename=filename,
        mime=mime,
        size_bytes=len(data),
        text_content=text,
    )
