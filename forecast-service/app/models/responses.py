from __future__ import annotations
from datetime import date, datetime
from pydantic import BaseModel, Field
import uuid


class HealthResponse(BaseModel):
    status: str
    version: str
    db_reachable: bool


class ForecastDataPoint(BaseModel):
    period: date
    value: float
    is_forecast: bool = False


class ForecastResponse(BaseModel):
    request_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    generated_at: datetime = Field(default_factory=datetime.utcnow)
    metric: str
    granularity: str
    method_used: str
    historical: list[ForecastDataPoint]
    forecast: list[ForecastDataPoint]
    insights: list[str]


class WhatIfPeriod(BaseModel):
    period: date
    value: float


class WhatIfResponse(BaseModel):
    request_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    generated_at: datetime = Field(default_factory=datetime.utcnow)
    scenario_label: str
    base_metric: str
    base_total: float
    projected_total: float
    growth_assumed: float        # monthly growth rate applied
    projected_periods: list[WhatIfPeriod]
    summary: str


class ErrorResponse(BaseModel):
    error: str
    detail: str
    request_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
