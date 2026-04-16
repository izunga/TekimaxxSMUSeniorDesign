#
# Section 5 — AI Integration Layer: Gamma Router
#
# Classifies every incoming question and routes it to:
#   (a) FORECAST path — calls forecast-service /forecast for numeric data queries
#   (b) LLM path      — IBM Granite via Ollama for explanatory/advisory questions
#
# Classification strategy:
#   1. Check ESCALATION patterns first (always LLM)
#   2. Check FORECAST patterns (simple data lookup → forecast-service)
#   3. Default → LLM
from __future__ import annotations

import logging
import re
from datetime import date, datetime, timezone
from enum import Enum

import httpx
from dateutil.relativedelta import relativedelta

from app.config import Settings
from app.models.responses import AnalyzeResponse
from app.services.guardrails import GuardrailViolation, validate_llm_output
from app.services.ollama_client import OllamaClient, OllamaModelError, OllamaUnavailableError

logger = logging.getLogger(__name__)


class RouteDecision(str, Enum):
    FORECAST = "forecast"
    LLM = "llm"


# Numeric / data-lookup patterns → route to forecast-service
_FORECAST_PATTERNS = [re.compile(p, re.IGNORECASE) for p in [
    r"\b(what('?s| is))\b.{0,40}\b(revenue|income|expenses?|profit|loss|balance)\b",
    r"\b(show|get|give|tell)\b.{0,30}\b(revenue|income|expenses?|profit|loss|balance)\b",
    r"\bhow much\b.{0,30}\b(have i|did i|i)\b.{0,20}\b(make|earn|spend|receive|pay)\b",
    r"\b(forecast|predict|project)\b.{0,30}\b(revenue|income|expenses?|profit)\b",
    r"\b(total|sum)\b.{0,30}\b(revenue|income|expenses?|profit|loss)\b",
    r"\bam i (currently )?(profitable|in the (red|black)|breaking even)\b",
    r"\b(how many|number of)\b.{0,30}\b(transactions?|payments?|charges?)\b",
]]

# Explanatory patterns — always go to LLM even if data keywords present
_ESCALATION_PATTERNS = [re.compile(p, re.IGNORECASE) for p in [
    r"\b(why|what('?s| is) causing|explain|reason|breakdown of)\b",
    r"\b(what should|how (should|can|do) i|advice|recommend|suggest|strategy)\b",
    r"\b(compare|versus|vs\.?|benchmark)\b",
    r"\b(risk|scenario|what if|if i|runway|burn rate)\b",
    r"\b(improve|optimize|grow|scale|reduce|cut)\b.{0,30}\b(revenue|expenses?|profit|cost)\b",
    r"\b(will i|can i|am i on track|going to)\b",
]]

_SYSTEM_PROMPT = """You are a financial advisor assistant for Tekimax, helping early-stage entrepreneurs
understand their business finances.

Guidelines:
- Explain financial concepts in plain, jargon-free language.
- Base answers ONLY on data provided in the conversation context.
- Never guarantee future performance or make investment recommendations.
- If uncertain about a figure, say so explicitly.
- Never repeat personal identifiers, account numbers, or sensitive data.
- Keep answers concise and actionable.
"""


def classify(question: str) -> RouteDecision:
    q = " ".join(question.strip().lower().split())
    for p in _ESCALATION_PATTERNS:
        if p.search(q):
            return RouteDecision.LLM
    for p in _FORECAST_PATTERNS:
        if p.search(q):
            return RouteDecision.FORECAST
    return RouteDecision.LLM


class GammaRouter:
    """Routes /analyze questions to forecast-service or IBM Granite."""

    def __init__(self, settings: Settings) -> None:
        self.settings = settings

    async def route(self, *, user_id: str, question: str, request_id: str) -> AnalyzeResponse:
        decision = classify(question)
        if decision is RouteDecision.FORECAST:
            return await self._answer_via_forecast(user_id=user_id, question=question, request_id=request_id)
        return await self._answer_via_llm(user_id=user_id, question=question, request_id=request_id)

    # ── Forecast path ─────────────────────────────────────────────────────────

    async def _answer_via_forecast(self, *, user_id: str, question: str, request_id: str) -> AnalyzeResponse:
        logger.info("gamma_router: FORECAST path request_id=%s", request_id)
        today = date.today()
        end = date(today.year, today.month, 1) - relativedelta(days=1)
        start = date(end.year, end.month, 1) - relativedelta(months=11)

        payload = {
            "user_id": user_id,
            "metric": _detect_metric(question),
            "start_date": start.isoformat(),
            "end_date": end.isoformat(),
            "granularity": "monthly",
            "periods_ahead": 3,
            "method": "auto",
        }

        data_context = None
        answer = ""
        try:
            async with httpx.AsyncClient(timeout=15.0) as client:
                resp = await client.post(
                    f"{self.settings.forecast_service_url}/forecast",
                    json=payload,
                    headers={"X-Tekimax-User-Id": user_id},
                )
                resp.raise_for_status()
                data = resp.json()
                data_context = data

                historical = data.get("historical", [])
                forecast_pts = data.get("forecast", [])
                insights = data.get("insights", [])
                metric = data.get("metric", "revenue")

                total = sum(p["value"] for p in historical)
                answer = f"Your total {metric} over the last 12 months was ${total:,.2f}. "
                if insights:
                    answer += insights[0]
                if forecast_pts:
                    next_val = forecast_pts[0]["value"]
                    answer += f" Forecast for next period: ${next_val:,.2f}."

        except Exception as exc:
            logger.warning("gamma_router: forecast-service call failed (%s) — falling back to LLM", exc)
            return await self._answer_via_llm(user_id=user_id, question=question, request_id=request_id)

        return AnalyzeResponse(
            request_id=request_id,
            generated_at=datetime.now(timezone.utc),
            question=question,
            route_taken=RouteDecision.FORECAST.value,
            answer=answer.strip(),
            data_context=data_context,
            model_used=None,
            tokens_used=None,
            guardrail_passed=True,
        )

    # ── LLM path ──────────────────────────────────────────────────────────────

    async def _answer_via_llm(self, *, user_id: str, question: str, request_id: str) -> AnalyzeResponse:
        logger.info("gamma_router: LLM path request_id=%s", request_id)
        client = OllamaClient(self.settings)
        try:
            raw = await client.chat(system_prompt=_SYSTEM_PROMPT, user_message=question)
            content = raw["message"]["content"]
            content = validate_llm_output(content, context="analyze.answer")
            return AnalyzeResponse(
                request_id=request_id,
                generated_at=datetime.now(timezone.utc),
                question=question,
                route_taken=RouteDecision.LLM.value,
                answer=content,
                model_used=self.settings.ollama_model,
                tokens_used=raw.get("eval_count"),
                guardrail_passed=True,
            )
        except GuardrailViolation as exc:
            raise   # propagate — endpoint's global handler returns 500
        except (OllamaUnavailableError, OllamaModelError) as exc:
            logger.warning("gamma_router: LLM unavailable (%s) — degraded response", exc)
            return AnalyzeResponse(
                request_id=request_id,
                generated_at=datetime.now(timezone.utc),
                question=question,
                route_taken="degraded",
                answer=(
                    "The AI advisor is temporarily unavailable. "
                    "Please ensure Ollama is running and IBM Granite is pulled: "
                    "`ollama pull granite3.3:8b`."
                ),
                model_used=None,
                tokens_used=None,
                guardrail_passed=True,
            )


def _detect_metric(question: str) -> str:
    q = question.lower()
    if any(k in q for k in ("expense", "spending", "spent", "cost")):
        return "expenses"
    if any(k in q for k in ("profit", "loss")):
        return "profit"
    if "balance" in q:
        return "balance"
    if "income" in q:
        return "net_income"
    return "revenue"
