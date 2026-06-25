# syntax=docker/dockerfile:1.7

FROM python:3.13-slim AS base
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_PROJECT_ENVIRONMENT=/app/.venv
WORKDIR /app

# System deps for local AI services:
#   - tesseract-ocr + lang packs    → image OCR via pytesseract
#   - libmagic1                     → libmagic-based mime sniffing (used by python-magic)
#   - ffmpeg                        → audio decoding for faster-whisper (mp3/m4a/webm → wav)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        tesseract-ocr \
        tesseract-ocr-fra \
        tesseract-ocr-eng \
        ffmpeg \
        libmagic1 \
    && rm -rf /var/lib/apt/lists/*

# Install uv
RUN pip install --no-cache-dir uv

FROM base AS deps
COPY apps/agent/pyproject.toml apps/agent/uv.lock* ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-install-project || uv sync --no-install-project

FROM deps AS build
COPY apps/agent /app
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-editable || uv sync --no-editable

FROM python:3.13-slim AS runtime
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PATH="/app/.venv/bin:$PATH"
WORKDIR /app
RUN groupadd -r app && useradd -r -g app app
COPY --from=build --chown=app:app /app /app
USER app
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]

FROM base AS dev
COPY apps/agent/pyproject.toml apps/agent/uv.lock* /app/
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync || true
EXPOSE 8000
CMD ["uv", "run", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000", "--reload"]
