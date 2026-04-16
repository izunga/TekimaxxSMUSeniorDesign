from __future__ import annotations
from datetime import date
from typing import Literal
from pydantic import BaseModel, Field, field_validator


class ForecastRequest(BaseModel):
    """POST /forecast — time-series forecast for a financial metric."""
    user_id: str = Field(..., min_length=1, max_length=128)
    metric: Literal["revenue", "expenses", "net_income", "profit", "balance", "refunds"] = "revenue"
    start_date: date
    end_date: date
    granularity: Literal["daily", "weekly", "monthly"] = "monthly"
    periods_ahead: int = Field(default=3, ge=1, le=24)
    method: Literal["moving_average", "exponential_smoothing", "regression", "auto"] = "auto"

    @field_validator("end_date")
    @classmethod
    def end_after_start(cls, v: date, info) -> date:
        if info.data.get("start_date") and v < info.data["start_date"]:
            raise ValueError("end_date must be on or after start_date")
        return v


class WhatIfScenario(BaseModel):
    growth_rate: float = Field(..., ge=-1.0, le=10.0,
                               description="Monthly growth rate, e.g. 0.10 for 10%")
    periods_ahead: int = Field(default=6, ge=1, le=36)
    label: str = Field(default="Scenario", max_length=64)


class WhatIfRequest(BaseModel):
    """POST /what-if — scenario analysis with assumed growth."""
    user_id: str = Field(..., min_length=1, max_length=128)
    base_metric: Literal["revenue", "expenses", "net_income", "profit", "balance"] = "revenue"
    start_date: date
    end_date: date
    scenario: WhatIfScenario

    @field_validator("end_date")
    @classmethod
    def end_after_start(cls, v: date, info) -> date:
        if info.data.get("start_date") and v < info.data["start_date"]:
            raise ValueError("end_date must be on or after start_date")
        return v
