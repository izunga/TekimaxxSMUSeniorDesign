from __future__ import annotations
import pytest
from datetime import date
from unittest.mock import patch, AsyncMock
from fastapi.testclient import TestClient

from app.main import app
from app.services import forecaster as fc
from app.services.mock_data import get_metric_by_period

HEADERS = {"X-Tekimax-User-Id": "usr_test"}


@pytest.fixture()
def client():
    with TestClient(app) as c:
        yield c


# ── Forecaster unit tests ─────────────────────────────────────────────────────

class TestMovingAverage:
    def test_returns_n_periods(self):
        values = [100.0] * 6
        periods = [date(2024, m, 1) for m in range(1, 7)]
        r = fc.moving_average(values, window=3, n=3, last_period=periods[-1])
        assert r is not None and len(r.periods) == 3

    def test_method_label(self):
        values = [100.0] * 6
        periods = [date(2024, m, 1) for m in range(1, 7)]
        r = fc.moving_average(values, n=2, last_period=periods[-1])
        assert r.method == "moving_average"

    def test_returns_none_below_min_points(self):
        assert fc.moving_average([100.0, 200.0], n=3, last_period=date(2024, 1, 1)) is None

    def test_forecast_dates_after_last_actual(self):
        values = [100.0] * 6
        last = date(2024, 6, 1)
        r = fc.moving_average(values, n=3, last_period=last)
        assert all(p > last for p in r.periods)


class TestLinearRegression:
    def _data(self, n=12):
        periods = [date(2024, m, 1) for m in range(1, n + 1)]
        values = [1000.0 + i * 100 for i in range(n)]
        return periods, values

    def test_returns_n_periods(self):
        p, v = self._data()
        r = fc.linear_regression(p, v, n=3)
        assert r is not None and len(r.periods) == 3

    def test_positive_slope_gives_increasing_forecast(self):
        p, v = self._data()
        r = fc.linear_regression(p, v, n=3)
        assert r.values[0] > v[-1]

    def test_method_label(self):
        p, v = self._data()
        assert fc.linear_regression(p, v, n=2).method == "regression"


class TestForecastDispatch:
    def _monthly(self, n=12):
        return [date(2024, m, 1) for m in range(1, n + 1)], [1000.0 * (1.05**i) for i in range(n)]

    def test_auto_returns_result(self):
        p, v = self._monthly()
        r = fc.forecast(periods=p, values=v, n=3, method="auto")
        assert r is not None

    def test_moving_average_method(self):
        p, v = self._monthly()
        r = fc.forecast(periods=p, values=v, n=3, method="moving_average")
        assert r.method == "moving_average"

    def test_regression_method(self):
        p, v = self._monthly()
        r = fc.forecast(periods=p, values=v, n=3, method="regression")
        assert r.method == "regression"

    def test_insufficient_data_returns_none(self):
        assert fc.forecast(periods=[date(2024, 1, 1)], values=[100.0], n=3) is None


class TestMockData:
    def test_monthly_12_rows(self):
        rows = get_metric_by_period(metric="revenue", start_date=date(2024, 1, 1),
                                    end_date=date(2024, 12, 31), granularity="monthly")
        assert len(rows) == 12

    def test_deterministic(self):
        kwargs = dict(metric="revenue", start_date=date(2024, 1, 1),
                      end_date=date(2024, 6, 30), granularity="monthly")
        assert [r["value"] for r in get_metric_by_period(**kwargs)] == \
               [r["value"] for r in get_metric_by_period(**kwargs)]

    def test_all_metrics_return_rows(self):
        for m in ["revenue", "refunds", "net_income", "expenses", "profit", "balance"]:
            rows = get_metric_by_period(metric=m, start_date=date(2024, 1, 1),
                                        end_date=date(2024, 3, 31), granularity="monthly")
            assert len(rows) > 0


# ── HTTP endpoint tests ───────────────────────────────────────────────────────

class TestForecastEndpoint:
    PAYLOAD = {
        "user_id": "usr_test",
        "metric": "revenue",
        "start_date": "2024-01-01",
        "end_date": "2024-12-31",
        "granularity": "monthly",
        "periods_ahead": 3,
        "method": "auto",
    }

    def _mock_fetch(self):
        from datetime import date
        rows = [{"period": date(2024, m, 1), "value": 8500.0 * (1.05 ** (m - 1))} for m in range(1, 13)]
        return patch("app.router.forecast.data_service.fetch_metric", new=AsyncMock(return_value=rows))

    def test_returns_200(self, client):
        with self._mock_fetch():
            resp = client.post("/forecast", json=self.PAYLOAD, headers=HEADERS)
        assert resp.status_code == 200

    def test_has_historical_and_forecast(self, client):
        with self._mock_fetch():
            body = client.post("/forecast", json=self.PAYLOAD, headers=HEADERS).json()
        assert len(body["historical"]) == 12
        assert len(body["forecast"]) == 3

    def test_forecast_dates_after_historical(self, client):
        with self._mock_fetch():
            body = client.post("/forecast", json=self.PAYLOAD, headers=HEADERS).json()
        last_actual = body["historical"][-1]["period"]
        first_forecast = body["forecast"][0]["period"]
        assert first_forecast > last_actual

    def test_insights_populated(self, client):
        with self._mock_fetch():
            body = client.post("/forecast", json=self.PAYLOAD, headers=HEADERS).json()
        assert len(body["insights"]) > 0

    def test_requires_auth_header(self, client):
        resp = client.post("/forecast", json=self.PAYLOAD)
        assert resp.status_code == 422

    def test_invalid_date_range_rejected(self, client):
        bad = {**self.PAYLOAD, "start_date": "2024-12-31", "end_date": "2024-01-01"}
        resp = client.post("/forecast", json=bad, headers=HEADERS)
        assert resp.status_code == 422

    def test_all_methods_accepted(self, client):
        for method in ["moving_average", "exponential_smoothing", "regression", "auto"]:
            with self._mock_fetch():
                resp = client.post("/forecast", json={**self.PAYLOAD, "method": method}, headers=HEADERS)
            assert resp.status_code == 200, f"method={method} failed"

    def test_empty_data_returns_200_with_empty_lists(self, client):
        with patch("app.router.forecast.data_service.fetch_metric", new=AsyncMock(return_value=[])):
            body = client.post("/forecast", json=self.PAYLOAD, headers=HEADERS).json()
        assert body["historical"] == [] and body["forecast"] == []
