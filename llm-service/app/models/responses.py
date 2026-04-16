from __future__ import annotations
from datetime import datetime
from typing import Any
from pydantic import BaseModel, Field
import uuid


class HealthResponse(BaseModel):
    status: str
    version: str
    ollama_reachable: bool


class InsightsResponse(BaseModel):
    request_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    generated_at: datetime = Field(default_factory=datetime.utcnow)
    insights: str
    model_used: str | None = None
    tokens_used: int | None = None
    guardrail_passed: bool = False


class AnalyzeResponse(BaseModel):
    request_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    generated_at: datetime = Field(default_factory=datetime.utcnow)
    question: str
    route_taken: str                  # "forecast" | "llm" | "degraded"
    answer: str
    data_context: dict[str, Any] | None = None   # forecast data when route="forecast"
    model_used: str | None = None
    tokens_used: int | None = None
    guardrail_passed: bool = False


class ErrorResponse(BaseModel):
    error: str
    detail: str
    request_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
