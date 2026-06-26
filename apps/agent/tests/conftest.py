import os

# app.settings instantiates a Settings() at import time with required
# DATABASE_URL / REDIS_URL fields. Provide harmless defaults so unit tests that
# import app modules don't need a live database or broker.
os.environ.setdefault("DATABASE_URL", "postgres://app:app@localhost:5432/app?sslmode=disable")
os.environ.setdefault("REDIS_URL", "redis://localhost:6379/0")
