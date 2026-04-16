from __future__ import annotations
import pytest
from unittest.mock import AsyncMock, patch, MagicMock
from fastapi.testclient import TestClient

from app.main import app
from app.services.gamma_router import RouteDecision, classify, _detect_metric

HEADERS = {"X-Tekimax-User-Id": "usr_test"}


@pytest.fixture()
def client():
    with TestClient(app) as c:
        yield c


def _mock_llm(content="Your finances look healthy."):
    return patch(
        "app.services.gamma_router.OllamaClient.chat",
        new=AsyncMock(return_value={
            "message": {"role": "assistant", "content": content},
            "model": "granite3.3:8b",
            "eval_count": 30,
            "done": True,
        })
    )


def _mock_forecast_http(value=8500.0):
    mock_resp = MagicMock()
    mock_resp.status_code = 200
    mock_resp.raise_for_status = MagicMock()
    mock_resp.json.return_value = {
        "metric": "revenue",
        "historical": [{"period": "2024-01-01", "value": value, "is_forecast": False}],
        "forecast": [{"period": "2025-01-01", "value": value * 1.08, "is_forecast": True}],
        "insights": ["Revenue is trending upward."],
        "method_used": "auto",
    }
    return patch(
        "app.services.gamma_router.httpx.AsyncClient",
        return_value=AsyncMock(__aenter__=AsyncMock(return_value=AsyncMock(
            post=AsyncMock(return_value=mock_resp)
        )), __aexit__=AsyncMock(return_value=False))
    )


# ── Classifier unit tests ─────────────────────────────────────────────────────

class TestGammaClassifier:

    def test_revenue_question_routes_to_forecast(self):
        assert classify("What is my revenue?") == RouteDecision.FORECAST

    def test_expenses_question_routes_to_forecast(self):
        assert classify("What are my expenses?") == RouteDecision.FORECAST

    def test_profitability_routes_to_forecast(self):
        assert classify("Am I currently profitable?") == RouteDecision.FORECAST

    def test_why_escalates_to_llm(self):
        assert classify("Why are my expenses so high?") == RouteDecision.LLM

    def test_strategy_escalates_to_llm(self):
        assert classify("What should I do to grow revenue?") == RouteDecision.LLM

    def test_on_track_escalates_to_llm(self):
        assert classify("Will I hit $100k by Q4?") == RouteDecision.LLM

    def test_compare_escalates_to_llm(self):
        assert classify("Compare my revenue to industry peers.") == RouteDecision.LLM

    def test_default_unknown_goes_to_llm(self):
        assert classify("Hello there") == RouteDecision.LLM


class TestDetectMetric:
    def test_expense_keywords(self):
        assert _detect_metric("what are my expenses?") == "expenses"

    def test_profit_keyword(self):
        assert _detect_metric("show me profit") == "profit"

    def test_balance_keyword(self):
        assert _detect_metric("what is my balance?") == "balance"

    def test_income_keyword(self):
        assert _detect_metric("what is my income?") == "net_income"

    def test_default_revenue(self):
        assert _detect_metric("how much did I make?") == "revenue"


# ── Analyze endpoint ──────────────────────────────────────────────────────────

class TestAnalyzeEndpoint:

    def test_numeric_question_routes_to_forecast(self, client):
        with _mock_forecast_http():
            body = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "What is my revenue?",
            }, headers=HEADERS).json()
        assert body["route_taken"] == "forecast"

    def test_forecast_route_populates_answer(self, client):
        with _mock_forecast_http(8500.0):
            body = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "What is my revenue?",
            }, headers=HEADERS).json()
        assert "$" in body["answer"]

    def test_forecast_route_includes_data_context(self, client):
        with _mock_forecast_http():
            body = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "What is my revenue?",
            }, headers=HEADERS).json()
        assert body["data_context"] is not None

    def test_advisory_question_routes_to_llm(self, client):
        with _mock_llm():
            body = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "What strategy should I use to grow revenue?",
            }, headers=HEADERS).json()
        assert body["route_taken"] == "llm"

    def test_llm_route_populates_model_used(self, client):
        with _mock_llm():
            body = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "Why are my expenses so high?",
            }, headers=HEADERS).json()
        assert body["model_used"] == "granite3.3:8b"

    def test_guardrail_passed_true_on_llm(self, client):
        with _mock_llm():
            body = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "Why is revenue declining?",
            }, headers=HEADERS).json()
        assert body["guardrail_passed"] is True

    def test_compliance_violation_returns_500(self, client):
        with _mock_llm(content="You will definitely earn guaranteed returns."):
            resp = client.post("/analyze", json={
                "user_id": "usr_test",
                "question": "Will I get rich?",
            }, headers=HEADERS)
        assert resp.status_code == 500

    def test_forecast_service_down_falls_back_to_llm(self, client):
        with patch("app.services.gamma_router.httpx.AsyncClient",
                   side_effect=Exception("connection refused")):
            with _mock_llm():
                body = client.post("/analyze", json={
                    "user_id": "usr_test",
                    "question": "What is my revenue?",
                }, headers=HEADERS).json()
        # Falls back to LLM when forecast-service is down
        assert body["route_taken"] in ("llm", "degraded")

    def test_requires_auth_header(self, client):
        resp = client.post("/analyze", json={"user_id": "usr_test", "question": "test?"})
        assert resp.status_code == 422

    def test_question_too_short_rejected(self, client):
        resp = client.post("/analyze", json={"user_id": "usr_test", "question": "hi"},
                           headers=HEADERS)
        assert resp.status_code == 422
