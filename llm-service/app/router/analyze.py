#
# Section 5 — POST /analyze
# AI Integration Layer entry point.
# Gamma Router classifies the question and routes to forecast-service or IBM Granite.
from __future__ import annotations

import logging
import uuid

from fastapi import APIRouter, Depends
from fastapi.responses import JSONResponse

from app.auth import require_user_id
from app.config import Settings, get_settings
from app.models.requests import AnalyzeRequest
from app.models.responses import AnalyzeResponse, ErrorResponse
from app.services.gamma_router import GammaRouter
from app.services.guardrails import GuardrailViolation

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/analyze", tags=["analyze"])


@router.post(
    "",
    response_model=AnalyzeResponse,
    responses={422: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="AI Integration Layer — route question to forecast or LLM",
    description=(
        "Gamma Router classifies the question: numeric data queries go to forecast-service, "
        "explanatory/advisory questions go to IBM Granite via Ollama."
    ),
)
async def analyze_endpoint(
    body: AnalyzeRequest,
    user_id: str = Depends(require_user_id),
    settings: Settings = Depends(get_settings),
) -> AnalyzeResponse:
    request_id = str(uuid.uuid4())
    # NOTE: do not log question content — may contain PII
    logger.info("analyze request_id=%s", request_id)

    gamma = GammaRouter(settings)
    try:
        return await gamma.route(user_id=user_id, question=body.question, request_id=request_id)
    except GuardrailViolation:
        return JSONResponse(
            status_code=500,
            content=ErrorResponse(
                error="guardrail_violation",
                detail="The generated response violated compliance safeguards.",
            ).model_dump(),
        )
