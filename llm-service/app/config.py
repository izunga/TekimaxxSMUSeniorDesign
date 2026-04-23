from functools import lru_cache
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    app_name: str = "Tekimax LLM Service"
    app_version: str = "1.0.0"
    debug: bool = False

    # IBM Granite via Ollama — no cloud LLMs (compliance)
    ollama_base_url: str = "http://host.docker.internal:11434"
    ollama_model: str = "granite3.3:8b"
    ollama_timeout_seconds: int = 60

    # Forecast service URL — used by Gamma Router for numeric queries
    forecast_service_url: str = "http://forecast-service:8001"

    log_level: str = "INFO"

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")


@lru_cache()
def get_settings() -> Settings:
    return Settings()
