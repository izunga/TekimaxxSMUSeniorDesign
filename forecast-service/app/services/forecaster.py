#
# Section 4 — deterministic financial forecasting models.
# Three methods: moving average, exponential smoothing, linear regression.
# "auto" picks the best method for the available data.
from __future__ import annotations

import logging
from dataclasses import dataclass
from datetime import date

import numpy as np
from dateutil.relativedelta import relativedelta

logger = logging.getLogger(__name__)

MIN_POINTS = 4   # minimum data points required for any forecast


@dataclass
class ForecastResult:
    periods: list[date]
    values: list[float]
    method: str


# ── helpers ───────────────────────────────────────────────────────────────────

def _next_periods(last: date, n: int, freq: str) -> list[date]:
    """Generate n future periods after `last` at the given frequency."""
    out = []
    for i in range(1, n + 1):
        if freq == "MS":
            out.append(last + relativedelta(months=i))
        elif freq == "W":
            from datetime import timedelta
            out.append(last + timedelta(weeks=i))
        else:
            from datetime import timedelta
            out.append(last + timedelta(days=i))
    return out


def _insights(periods: list[date], values: list[float], method: str, forecast: ForecastResult | None) -> list[str]:
    if not values:
        return ["No data available for the selected period."]
    out: list[str] = []
    pct = round(((values[-1] - values[0]) / values[0]) * 100, 1) if values[0] else 0.0
    direction = "grew" if pct >= 0 else "declined"
    out.append(f"Over the period, the metric {direction} {abs(pct)}% "
               f"(from ${values[0]:,.2f} to ${values[-1]:,.2f}).")
    if len(values) >= 2:
        mom_changes = [(values[i] - values[i - 1]) / values[i - 1] * 100
                       for i in range(1, len(values)) if values[i - 1] != 0]
        if mom_changes:
            avg_mom = round(sum(mom_changes) / len(mom_changes), 1)
            out.append(f"Average period-over-period change: {avg_mom:+.1f}%.")
    if forecast:
        pct_next = round(((forecast.values[0] - values[-1]) / values[-1]) * 100, 1) if values[-1] else 0.0
        out.append(f"Forecast (next period, {method}): ${forecast.values[0]:,.2f} "
                   f"({pct_next:+.1f}% vs current).")
    return out


# ── Model 1: Moving Average ───────────────────────────────────────────────────

def moving_average(values: list[float], window: int = 3, n: int = 3,
                   last_period: date | None = None, freq: str = "MS") -> ForecastResult | None:
    if len(values) < MIN_POINTS:
        return None
    w = min(window, len(values))
    avg = sum(values[-w:]) / w
    periods = _next_periods(last_period or date.today(), n, freq)
    return ForecastResult(
        periods=periods,
        values=[round(avg, 2)] * n,
        method="moving_average",
    )


# ── Model 2: Exponential Smoothing ───────────────────────────────────────────

def exponential_smoothing(periods: list[date], values: list[float], n: int = 3,
                          freq: str = "MS") -> ForecastResult | None:
    if len(values) < MIN_POINTS:
        return None
    try:
        import pandas as pd
        from statsmodels.tsa.holtwinters import ExponentialSmoothing

        idx = pd.date_range(start=periods[0], periods=len(periods), freq=freq)
        series = pd.Series(values, index=idx)
        trend = "add" if len(values) >= 4 else None
        seasonal = "add" if len(values) >= 24 else None
        seasonal_periods = 12 if seasonal else None

        model = ExponentialSmoothing(
            series, trend=trend, seasonal=seasonal,
            seasonal_periods=seasonal_periods,
        ).fit(optimized=True)

        fcast = model.forecast(n)
        future_periods = _next_periods(periods[-1], n, freq)
        return ForecastResult(
            periods=future_periods,
            values=[round(max(v, 0.0), 2) for v in fcast.values],
            method="exponential_smoothing",
        )
    except Exception as exc:
        logger.warning("exponential_smoothing failed (%s), falling back to moving_average", exc)
        return moving_average(values, n=n, last_period=periods[-1], freq=freq)


# ── Model 3: Linear Regression ───────────────────────────────────────────────

def linear_regression(periods: list[date], values: list[float], n: int = 3,
                      freq: str = "MS") -> ForecastResult | None:
    if len(values) < MIN_POINTS:
        return None
    x = np.arange(len(values), dtype=float)
    slope, intercept = np.polyfit(x, values, 1)
    future_x = np.arange(len(values), len(values) + n, dtype=float)
    future_vals = [round(float(np.polyval([slope, intercept], xi)), 2) for xi in future_x]
    future_periods = _next_periods(periods[-1], n, freq)
    return ForecastResult(
        periods=future_periods,
        values=future_vals,
        method="regression",
    )


# ── Auto-select ───────────────────────────────────────────────────────────────

def forecast(
    *,
    periods: list[date],
    values: list[float],
    n: int = 3,
    freq: str = "MS",
    method: str = "auto",
) -> ForecastResult | None:
    """
    Run the requested forecasting method. Returns None if insufficient data.
    `method`: "moving_average" | "exponential_smoothing" | "regression" | "auto"
    """
    if len(values) < MIN_POINTS:
        logger.info("forecast: only %d points, minimum is %d — skipping", len(values), MIN_POINTS)
        return None

    if method == "moving_average":
        return moving_average(values, n=n, last_period=periods[-1], freq=freq)
    if method == "regression":
        return linear_regression(periods, values, n=n, freq=freq)
    if method == "exponential_smoothing":
        return exponential_smoothing(periods, values, n=n, freq=freq)

    # auto: prefer exponential smoothing; fall back to moving average on error
    result = exponential_smoothing(periods, values, n=n, freq=freq)
    return result or moving_average(values, n=n, last_period=periods[-1], freq=freq)


def compute_insights(
    periods: list[date], values: list[float], method: str, result: ForecastResult | None
) -> list[str]:
    return _insights(periods, values, method, result)
