#
# Deterministic mock data — enables demo without Person 1's DB.
# Enabled via USE_MOCK_DATA=true in environment.
from __future__ import annotations

import random
from datetime import date, timedelta
from typing import Any

from dateutil.relativedelta import relativedelta

_BASE = {"revenue": 8_500.0, "refunds": 320.0, "net_income": 8_180.0,
         "expenses": 4_200.0, "profit": 3_980.0, "balance": 15_000.0}
_GROWTH = {"revenue": 1.08, "refunds": 1.02, "net_income": 1.09,
           "expenses": 1.03, "profit": 1.10, "balance": 1.06}


def _rng(metric: str, start: date, gran: str) -> random.Random:
    seed = hash((metric, start.isoformat(), gran)) & 0xFFFF_FFFF
    return random.Random(seed)


def get_metric_by_period(
    *, metric: str, start_date: date, end_date: date, granularity: str
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    rng = _rng(metric, start_date, granularity)
    base = _BASE.get(metric, 8_000.0)
    growth = _GROWTH.get(metric, 1.05)

    if granularity == "monthly":
        cur = date(start_date.year, start_date.month, 1)
        end = date(end_date.year, end_date.month, 1)
        while cur <= end:
            rows.append({"period": cur, "value": round(base * rng.uniform(0.93, 1.07), 2)})
            base *= growth
            cur += relativedelta(months=1)
    elif granularity == "weekly":
        cur = start_date - timedelta(days=start_date.weekday())
        base /= 4.33
        while cur <= end_date:
            rows.append({"period": cur, "value": round(base * rng.uniform(0.90, 1.10), 2)})
            base *= growth ** (1 / 4.33)
            cur += timedelta(weeks=1)
    else:
        cur = start_date
        base /= 30.0
        while cur <= end_date:
            rows.append({"period": cur, "value": round(base * rng.uniform(0.80, 1.20), 2)})
            base *= growth ** (1 / 30.0)
            cur += timedelta(days=1)

    return rows


def get_total_for_period(*, start_date: date, end_date: date) -> float:
    return 71_300.0
