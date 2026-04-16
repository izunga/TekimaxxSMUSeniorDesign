#
# Section 5 — POST /analyze
# AI Integration Layer entry point.
# Gamma Router classifies the question and routes to forecast-service or IBM Granite.
from __future__ import annotations

import logging
import uuid

from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.config import Settings, get_settings
from app.models.requests import AnalyzeRequest
from app.models.responses import AnalyzeResponse, ErrorResponse
from app.services.gamma_router import GammaRouter

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/analyze", tags=["analyze"])


def _require_user(x_tekimax_user_id: str = Header(..., alias="X-Tekimax-User-Id")) -> str:
    if not x_tekimax_user_id.strip():
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED,
                            detail="Missing authenticated user identity")
    return x_tekimax_user_id.strip()


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
    user_id: str = Depends(_require_user),
    settings: Settings = Depends(get_settings),
) -> AnalyzeResponse:
    request_id = str(uuid.uuid4())
    # NOTE: do not log question content — may contain PII
    logger.info("analyze request_id=%s", request_id)

    gamma = GammaRouter(settings)
    return await gamma.route(user_id=user_id, question=body.question, request_id=request_id)
