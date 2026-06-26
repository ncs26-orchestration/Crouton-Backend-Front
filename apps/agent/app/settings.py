from pydantic import Field, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    database_url: str = Field(alias="DATABASE_URL")
    redis_url: str = Field(alias="REDIS_URL")

    anthropic_api_key: str | None = Field(default=None, alias="ANTHROPIC_API_KEY")
    openai_api_key: str | None = Field(default=None, alias="OPENAI_API_KEY")
    google_api_key: str | None = Field(default=None, alias="GOOGLE_API_KEY")
    groq_api_key: str | None = Field(default=None, alias="GROQ_API_KEY")

    # DeepSeek — OpenAI-compatible. `deepseek-v4-flash` is fast + cheap and
    # follows JSON well; the agents use it for real per-request reasoning.
    deepseek_api_key: str | None = Field(default=None, alias="DEEPSEEK_API_KEY")
    deepseek_base_url: str = Field(default="https://api.deepseek.com", alias="DEEPSEEK_BASE_URL")
    deepseek_model: str = Field(default="deepseek-v4-flash", alias="DEEPSEEK_MODEL")

    default_model: str = Field(default="claude-opus-4-7", alias="AGENT_DEFAULT_MODEL")

    # Extractor stack:
    #   "ollama"    — local model via Ollama REST API
    #   "groq"      — GroqCloud (free/fast inference). Needs GROQ_API_KEY.
    #   "gemini"    — Google Gemini Flash. Needs GOOGLE_API_KEY.
    #   "anthropic" — Claude. Needs ANTHROPIC_API_KEY.
    #
    # Auto-selection priority: GROQ_API_KEY > GOOGLE_API_KEY > ANTHROPIC_API_KEY > ollama
    extractor_provider: str = Field(
        default="ollama",
        alias="AGENT_EXTRACTOR_PROVIDER",
    )  # "ollama" | "groq" | "gemini" | "anthropic"
    extractor_model: str = Field(
        default="qwen2.5:3b",
        alias="AGENT_EXTRACTOR_MODEL",
    )
    ollama_base_url: str = Field(
        default="http://host.docker.internal:11434",
        alias="OLLAMA_BASE_URL",
    )

    @model_validator(mode="after")
    def auto_select_extractor_provider(self) -> "Settings":
        """Auto-select extractor provider based on available API keys.

        Priority: GROQ_API_KEY > GOOGLE_API_KEY > ANTHROPIC_API_KEY > ollama
        """
        explicit_provider = self.extractor_provider != "ollama"

        if not explicit_provider:
            if self.groq_api_key and self.groq_api_key.strip():
                self.extractor_provider = "groq"
                self.extractor_model = "llama-3.3-70b-versatile"
            elif self.google_api_key and self.google_api_key.strip():
                self.extractor_provider = "gemini"
                self.extractor_model = "gemini-2.0-flash"
            elif self.anthropic_api_key and self.anthropic_api_key.strip():
                self.extractor_provider = "anthropic"
                if not self.default_model or "claude" not in self.default_model:
                    self.default_model = "claude-3-haiku-20240307"

        return self

    # Local ASR (Whisper) — runs in-process via faster-whisper, no
    # network egress. Models are downloaded once and cached in the
    # `whisper_cache` named volume (see compose.override).
    #
    # Trade-offs:
    #   tiny    ~75 MB,  fastest, OK for short clear utterances
    #   base    ~150 MB, balanced — default
    #   small   ~500 MB, better French/Arabic accuracy, slower
    whisper_model: str = Field(default="base", alias="WHISPER_MODEL")
    whisper_cache_dir: str = Field(default="/app/.whisper-cache", alias="WHISPER_CACHE_DIR")
    # Compute type: "int8" is the CPU-friendly default; "float16" is
    # a noticeable speedup on AVX2/AVX512 hosts. "auto" lets
    # faster-whisper pick.
    whisper_compute_type: str = Field(default="int8", alias="WHISPER_COMPUTE_TYPE")

    app_env: str = Field(default="development", alias="APP_ENV")


settings = Settings()
