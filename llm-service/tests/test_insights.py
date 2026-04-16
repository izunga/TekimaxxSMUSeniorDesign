from __future__ import annotations
import pytest
from unittest.mock import AsyncMock, patch
from fastapi.testclient import TestClient

from app.main import app

HEADERS = {"X-Tekimax-User-Id": "usr_test"}
PAYLOAD = {
    "user_id": "usr_test",
    "financial_data": {
        "metric": "revenue",
        "period": "2024-Q4",
        "value": 35000.0,
        "trend": "increasing",
    },
}

_MOCK_RESPONSE = {
    "message": {"role": "assistant", "content": "Your Q4 revenue of $35,000 shows healthy growth."},
    "model": "granite3.3:8b",
    "eval_count": 50,
    "done": True,
}


@pytest.fixture()
def client():
    with TestClient(app) as c:
        yield c


def _mock_chat(content="Your Q4 revenue of $35,000 shows healthy growth.", eval_count=50):
    return patch(
        "app.router.insights.OllamaClient.chat",
        new=AsyncMock(return_value={
            "message": {"role": "assistant", "content": content},
            "model": "granite3.3:8b",
            "eval_count": eval_count,
            "done": True,
        })
    )


class TestInsightsEndpoint:

    def test_returns_200(self, client):
        with _mock_chat():
            resp = client.post("/insights", json=PAYLOAD, headers=HEADERS)
        assert resp.status_code == 200

    def test_insights_populated(self, client):
        with _mock_chat():
            body = client.post("/insights", json=PAYLOAD, headers=HEADERS).json()
        assert len(body["insights"]) > 0

    def test_model_used_populated(self, client):
        with _mock_chat():
            body = client.post("/insights", json=PAYLOAD, headers=HEADERS).json()
        assert body["model_used"] == "granite3.3:8b"

    def test_tokens_used_populated(self, client):
        with _mock_chat():
            body = client.post("/insights", json=PAYLOAD, headers=HEADERS).json()
        assert body["tokens_used"] == 50

    def test_guardrail_passed_true(self, client):
        with _mock_chat():
            body = client.post("/insights", json=PAYLOAD, headers=HEADERS).json()
        assert body["guardrail_passed"] is True

    def test_pii_stripped_from_response(self, client):
        with _mock_chat(content="Contact user@example.com for details."):
            body = client.post("/insights", json=PAYLOAD, headers=HEADERS).json()
        assert "user@example.com" not in body["insights"]
        assert "[REDACTED]" in body["insights"]

    def test_compliance_violation_returns_500(self, client):
        with _mock_chat(content="This is a guaranteed return of 100%."):
            resp = client.post("/insights", json=PAYLOAD, headers=HEADERS)
        assert resp.status_code == 500

    def test_ollama_down_returns_fallback(self, client):
        from app.services.ollama_client import OllamaUnavailableError
        with patch("app.router.insights.OllamaClient.chat", side_effect=OllamaUnavailableError("down")):
            body = client.post("/insights", json=PAYLOAD, headers=HEADERS).json()
        assert "unavailable" in body["insights"].lower() or "$35,000" in body["insights"]

    def test_requires_auth_header(self, client):
        resp = client.post("/insights", json=PAYLOAD)
        assert resp.status_code == 422

    def test_with_focus_question(self, client):
        payload = {**PAYLOAD, "question": "Is this growth sustainable?"}
        with _mock_chat():
            resp = client.post("/insights", json=payload, headers=HEADERS)
        assert resp.status_code == 200
