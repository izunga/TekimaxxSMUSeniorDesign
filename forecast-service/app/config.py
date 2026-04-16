from functools import lru_cache
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    app_name: str = "Tekimax Forecast Service"
    app_version: str = "1.0.0"
    debug: bool = False

    # Database — Person 1's PostgreSQL ledger (READ only)
    # Credentials from Person 4's docker-compose
    database_url: str = "postgresql+asyncpg://user:password@postgres:5432/ledger"

    # Forecasting defaults
    forecast_periods: int = 3        # default periods ahead
    moving_avg_window: int = 3       # window size for moving average

    # Demo mode — bypass DB with realistic mock data
    use_mock_data: bool = False

    log_level: str = "INFO"

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")


@lru_cache()
def get_settings() -> Settings:
    return Settings()
