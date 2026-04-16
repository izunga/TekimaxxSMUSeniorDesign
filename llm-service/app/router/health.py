from fastapi import APIRouter
from app.config import get_settings
from app.models.responses import HealthResponse
from app.services.ollama_client import OllamaClient

router = APIRouter(tags=["health"])


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    settings = get_settings()
    client = OllamaClient(settings)
    ollama_ok = await client.is_available()
    return HealthResponse(status="ok", version=settings.app_version, ollama_reachable=ollama_ok)
