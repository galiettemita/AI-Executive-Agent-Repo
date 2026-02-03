#loads environment variables (keys, DB url)
# loads serpAPI settings

from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    DATABASE_URL: str = "sqlite:///./app.db"
    JWT_SECRET: str = "dev_only_change_me"

    # SerpAPI (Discovery)
    SERPAPI_API_KEY: str | None = None
    SERPAPI_ENGINE: str = "google"
    SERPAPI_GL: str = "us"
    SERPAPI_HL: str = "en"

settings = Settings()
