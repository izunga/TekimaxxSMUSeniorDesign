import logging
import sys

from fastapi import FastAPI, Request, status
from fastapi.responses import JSONResponse

from app.config import get_settings
from app.models.responses import ErrorResponse
from app.router import analyze, health, insights

settings = get_settings()

logging.basicConfig(
    stream=sys.stdout, level=settings.log_level,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(__name__)


def create_app() -> FastAPI:
    app = FastAPI(
        title=settings.app_name,
        version=settings.app_version,
        description=(
            "LLM service powered by IBM Granite via Ollama. "
            "Provides /insights (explain financial data) and /analyze (AI Integration Layer)."
        ),
        docs_url="/docs",
    )

    @app.exception_handler(Exception)
    async def _global(request: Request, exc: Exception) -> JSONResponse:
        logger.error("Unhandled error on %s %s", request.method, request.url.path, exc_info=exc)
        return JSONResponse(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            content=ErrorResponse(error="internal_error",
                                  detail="An unexpected error occurred.").model_dump(),
        )

    app.include_router(health.router)
    app.include_router(insights.router)
    app.include_router(analyze.router)
    logger.info("LLM service v%s starting", settings.app_version)
    return app


app = create_app()
