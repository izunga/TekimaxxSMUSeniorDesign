from fastapi import APIRouter
from app.config import get_settings
from app.models.responses import HealthResponse

router = APIRouter(tags=["health"])


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    settings = get_settings()
    db_ok = False
    if not settings.use_mock_data:
        try:
            from app.db.connection import get_engine
            from sqlalchemy import text
            engine = get_engine(settings.database_url)
            async with engine.connect() as conn:
                await conn.execute(text("SELECT 1"))
            db_ok = True
        except Exception:
            db_ok = False
    else:
        db_ok = True   # mock mode: no DB needed
    return HealthResponse(status="ok", version=settings.app_version, db_reachable=db_ok)
