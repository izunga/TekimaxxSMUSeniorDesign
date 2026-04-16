#
# Section 3 — POST /insights
# Sends structured financial data to IBM Granite and returns a plain-language explanation.
# Guardrails applied unconditionally before the response is returned.
from __future__ import annotations

import logging
import uuid

from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.config import Settings, get_settings
from app.models.requests import InsightsRequest
from app.models.responses import ErrorResponse, InsightsResponse
from app.services.guardrails import GuardrailViolation, validate_llm_output
from app.services.ollama_client import OllamaClient, OllamaModelError, OllamaUnavailableError

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/insights", tags=["insights"])

_SYSTEM_PROMPT = """You are a financial data interpreter for Tekimax.
You receive structured financial data and explain it in plain language for non-finance founders.

Rules:
- Use simple, jargon-free language.
- Be factual — only describe what the data shows.
- Never guarantee future performance.
- Never repeat raw numbers beyond what's given.
- Keep the explanation under 150 words.
"""


def _require_user(x_tekimax_user_id: str = Header(..., alias="X-Tekimax-User-Id")) -> str:
    if not x_tekimax_user_id.strip():
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED,
                            detail="Missing authenticated user identity")
    return x_tekimax_user_id.strip()


@router.post(
    "",
    response_model=InsightsResponse,
    responses={422: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="Generate LLM insights from financial data",
    description="Accepts structured financial data and returns a plain-language explanation via IBM Granite.",
)
async def insights_endpoint(
    body: InsightsRequest,
    user_id: str = Depends(_require_user),
    settings: Settings = Depends(get_settings),
) -> InsightsResponse:
    request_id = str(uuid.uuid4())
    logger.info("insights request_id=%s metric=%s", request_id, body.financial_data.metric)

    # Build the user message from structured data
    fd = body.financial_data
    user_msg = (
        f"Metric: {fd.metric}\n"
        f"Period: {fd.period}\n"
        f"Value: ${fd.value:,.2f}\n"
        f"Trend: {fd.trend}\n"
    )
    if fd.additional_context:
        user_msg += f"Context: {fd.additional_context}\n"
    if body.question:
        user_msg += f"\nFocus question: {body.question}"

    client = OllamaClient(settings)
    try:
        raw = await client.chat(system_prompt=_SYSTEM_PROMPT, user_message=user_msg)
        content = raw["message"]["content"]
        content = validate_llm_output(content, context="insights.answer")

        return InsightsResponse(
            request_id=request_id,
            insights=content,
            model_used=settings.ollama_model,
            tokens_used=raw.get("eval_count"),
            guardrail_passed=True,
        )

    except GuardrailViolation:
        raise   # global handler → 500

    except (OllamaUnavailableError, OllamaModelError) as exc:
        logger.warning("insights: LLM unavailable (%s)", exc)
        return InsightsResponse(
            request_id=request_id,
            insights=(
                f"Your {fd.metric} for {fd.period} was ${fd.value:,.2f} "
                f"with a {fd.trend} trend. "
                "(AI explanation unavailable — Ollama/Granite not running.)"
            ),
            model_used=None,
            tokens_used=None,
            guardrail_passed=True,
        )
