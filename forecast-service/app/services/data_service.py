#
# Fetches time-series data from Postgres or mock layer depending on settings.
from __future__ import annotations

from datetime import date
from typing import Any

from app.config import Settings
from app.db.connection import get_engine
from app.db.repository import LedgerRepository
from app.services import mock_data


async def fetch_metric(
    *,
    settings: Settings,
    user_id: str,
    metric: str,
    start_date: date,
    end_date: date,
    granularity: str,
) -> list[dict[str, Any]]:
    if settings.use_mock_data:
        return mock_data.get_metric_by_period(
            metric=metric, start_date=start_date, end_date=end_date, granularity=granularity
        )
    engine = get_engine(settings.database_url)
    repo = LedgerRepository(engine)
    return await repo.get_metric_by_period(
        user_id=user_id, metric=metric,
        start_date=start_date, end_date=end_date, granularity=granularity,
    )


async def fetch_total(
    *, settings: Settings, user_id: str, start_date: date, end_date: date
) -> float:
    if settings.use_mock_data:
        return mock_data.get_total_for_period(start_date=start_date, end_date=end_date)
    engine = get_engine(settings.database_url)
    repo = LedgerRepository(engine)
    return await repo.get_total_for_period(
        user_id=user_id, start_date=start_date, end_date=end_date
    )
