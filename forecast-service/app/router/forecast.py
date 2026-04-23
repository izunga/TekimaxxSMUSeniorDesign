#
# Section 4 — POST /forecast
# Runs moving average, exponential smoothing, or regression on ledger data.
from __future__ import annotations

import logging
import uuid

from fastapi import APIRouter, Depends

from app.auth import require_user_id
from app.config import Settings, get_settings
from app.models.requests import ForecastRequest
from app.models.responses import ErrorResponse, ForecastDataPoint, ForecastResponse
from app.services import data_service, forecaster as fc

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/forecast", tags=["forecast"])

_FREQ = {"daily": "D", "weekly": "W", "monthly": "MS"}


@router.post(
    "",
    response_model=ForecastResponse,
    responses={422: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="Time-series financial forecast",
    description=(
        "Returns historical data points plus a forward forecast using the selected model. "
        "Methods: moving_average | exponential_smoothing | regression | auto."
    ),
)
async def forecast_endpoint(
    body: ForecastRequest,
    user_id: str = Depends(require_user_id),
    settings: Settings = Depends(get_settings),
) -> ForecastResponse:
    request_id = str(uuid.uuid4())
    logger.info("forecast request_id=%s metric=%s method=%s", request_id, body.metric, body.method)

    # 1. Fetch historical data
    rows = await data_service.fetch_metric(
        settings=settings,
        user_id=user_id,
        metric=body.metric,
        start_date=body.start_date,
        end_date=body.end_date,
        granularity=body.granularity,
    )

    if not rows:
        return ForecastResponse(
            request_id=request_id,
            metric=body.metric,
            granularity=body.granularity,
            method_used=body.method,
            historical=[],
            forecast=[],
            insights=["No data found for the selected period."],
        )

    periods = [r["period"] for r in rows]
    values = [r["value"] for r in rows]
    freq = _FREQ.get(body.granularity, "MS")

    # 2. Run forecast
    result = fc.forecast(
        periods=periods, values=values, n=body.periods_ahead, freq=freq, method=body.method
    )
    method_used = result.method if result else body.method

    # 3. Build response
    historical = [ForecastDataPoint(period=p, value=v, is_forecast=False)
                  for p, v in zip(periods, values)]
    forecast_pts = (
        [ForecastDataPoint(period=p, value=v, is_forecast=True)
         for p, v in zip(result.periods, result.values)]
        if result else []
    )
    insights = fc.compute_insights(periods, values, method_used, result)

    return ForecastResponse(
        request_id=request_id,
        metric=body.metric,
        granularity=body.granularity,
        method_used=method_used,
        historical=historical,
        forecast=forecast_pts,
        insights=insights,
    )
