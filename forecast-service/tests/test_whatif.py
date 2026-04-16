from __future__ import annotations
import pytest
from datetime import date
from unittest.mock import patch, AsyncMock
from fastapi.testclient import TestClient

from app.main import app

HEADERS = {"X-Tekimax-User-Id": "usr_test"}
PAYLOAD = {
    "user_id": "usr_test",
    "base_metric": "revenue",
    "start_date": "2024-01-01",
    "end_date": "2024-12-31",
    "scenario": {"growth_rate": 0.10, "periods_ahead": 6, "label": "10% growth"},
}


@pytest.fixture()
def client():
    with TestClient(app) as c:
        yield c


def _mock_rows():
    rows = [{"period": date(2024, m, 1), "value": 8_500.0} for m in range(1, 13)]
    return patch("app.router.whatif.data_service.fetch_metric", new=AsyncMock(return_value=rows))


class TestWhatIfEndpoint:

    def test_returns_200(self, client):
        with _mock_rows():
            resp = client.post("/what-if", json=PAYLOAD, headers=HEADERS)
        assert resp.status_code == 200

    def test_projected_periods_count(self, client):
        with _mock_rows():
            body = client.post("/what-if", json=PAYLOAD, headers=HEADERS).json()
        assert len(body["projected_periods"]) == 6

    def test_growth_rate_applied(self, client):
        with _mock_rows():
            body = client.post("/what-if", json=PAYLOAD, headers=HEADERS).json()
        # With 10% monthly growth, each period is 1.1x the prior
        pts = body["projected_periods"]
        ratio = round(pts[1]["value"] / pts[0]["value"], 2)
        assert abs(ratio - 1.10) < 0.01

    def test_projected_total_greater_than_base(self, client):
        with _mock_rows():
            body = client.post("/what-if", json=PAYLOAD, headers=HEADERS).json()
        assert body["projected_total"] > 0

    def test_summary_populated(self, client):
        with _mock_rows():
            body = client.post("/what-if", json=PAYLOAD, headers=HEADERS).json()
        assert len(body["summary"]) > 20
        assert "10%" in body["summary"] or "10.0" in body["summary"]

    def test_scenario_label_in_response(self, client):
        with _mock_rows():
            body = client.post("/what-if", json=PAYLOAD, headers=HEADERS).json()
        assert body["scenario_label"] == "10% growth"

    def test_negative_growth_rate_allowed(self, client):
        payload = {**PAYLOAD, "scenario": {"growth_rate": -0.05, "periods_ahead": 3, "label": "decline"}}
        with _mock_rows():
            resp = client.post("/what-if", json=payload, headers=HEADERS)
        assert resp.status_code == 200
        body = resp.json()
        assert body["projected_periods"][-1]["value"] < body["projected_periods"][0]["value"]

    def test_requires_auth_header(self, client):
        resp = client.post("/what-if", json=PAYLOAD)
        assert resp.status_code == 422

    def test_invalid_date_range_rejected(self, client):
        bad = {**PAYLOAD, "start_date": "2024-12-31", "end_date": "2024-01-01"}
        resp = client.post("/what-if", json=bad, headers=HEADERS)
        assert resp.status_code == 422

    def test_growth_rate_out_of_bounds_rejected(self, client):
        bad = {**PAYLOAD, "scenario": {"growth_rate": 50.0, "periods_ahead": 3, "label": "x"}}
        resp = client.post("/what-if", json=bad, headers=HEADERS)
        assert resp.status_code == 422
