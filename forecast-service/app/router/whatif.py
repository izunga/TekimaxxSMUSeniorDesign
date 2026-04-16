#
# Section 4 — POST /what-if
# Scenario analysis: applies an assumed growth rate to historical data
# and projects forward. No ML model — deterministic arithmetic projection.
from __future__ import annotations

import logging
import uuid

from dateutil.relativedelta import relativedelta
from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.config import Settings, get_settings
from app.models.requests import WhatIfRequest
from app.models.responses import ErrorResponse, WhatIfPeriod, WhatIfResponse
from app.services import data_service

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/what-if", tags=["what-if"])


def _require_user(x_tekimax_user_id: str = Header(..., alias="X-Tekimax-User-Id")) -> str:
    if not x_tekimax_user_id.strip():
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED,
                            detail="Missing authenticated user identity")
    return x_tekimax_user_id.strip()


@router.post(
    "",
    response_model=WhatIfResponse,
    responses={422: {"model": ErrorResponse}, 500: {"model": ErrorResponse}},
    summary="What-if scenario analysis",
    description=(
        "Fetches historical totals for a metric, then projects forward using the assumed "
        "monthly growth_rate for the requested number of periods."
    ),
)
async def whatif_endpoint(
    body: WhatIfRequest,
    user_id: str = Depends(_require_user),
    settings: Settings = Depends(get_settings),
) -> WhatIfResponse:
    request_id = str(uuid.uuid4())
    logger.info("what-if request_id=%s metric=%s growth=%.2f%%",
                request_id, body.base_metric, body.scenario.growth_rate * 100)

    # 1. Fetch current-period data for base total
    rows = await data_service.fetch_metric(
        settings=settings,
        user_id=user_id,
        metric=body.base_metric,
        start_date=body.start_date,
        end_date=body.end_date,
        granularity="monthly",
    )

    base_total = round(sum(r["value"] for r in rows), 2) if rows else 0.0

    # 2. Project forward — compound growth from last actual period
    scenario = body.scenario
    last_period = rows[-1]["period"] if rows else body.end_date
    last_value = rows[-1]["value"] if rows else base_total

    projected: list[WhatIfPeriod] = []
    running = last_value
    for i in range(1, scenario.periods_ahead + 1):
        running = round(running * (1 + scenario.growth_rate), 2)
        period = last_period + relativedelta(months=i)
        projected.append(WhatIfPeriod(period=period, value=running))

    projected_total = round(sum(p.value for p in projected), 2)

    # 3. Plain-language summary
    pct = round(scenario.growth_rate * 100, 1)
    direction = "grow" if scenario.growth_rate >= 0 else "decline"
    summary = (
        f"'{scenario.label}': at {pct:+.1f}% monthly growth, {body.base_metric} is projected to "
        f"{direction} from ${last_value:,.2f} to ${projected[-1].value:,.2f} "
        f"over {scenario.periods_ahead} months (total: ${projected_total:,.2f})."
    )

    return WhatIfResponse(
        request_id=request_id,
        scenario_label=scenario.label,
        base_metric=body.base_metric,
        base_total=base_total,
        projected_total=projected_total,
        growth_assumed=scenario.growth_rate,
        projected_periods=projected,
        summary=summary,
    )
