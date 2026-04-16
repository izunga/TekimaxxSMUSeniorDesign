from __future__ import annotations
from pydantic import BaseModel, Field


class FinancialContext(BaseModel):
    """Structured financial data passed to the LLM for explanation."""
    metric: str
    period: str                      # e.g. "2024-Q4" or "Jan 2025"
    value: float
    trend: str = "unknown"           # increasing | decreasing | stable | unknown
    additional_context: str | None = None


class InsightsRequest(BaseModel):
    """POST /insights — explain a financial data snapshot in plain language."""
    user_id: str = Field(..., min_length=1, max_length=128)
    financial_data: FinancialContext
    question: str | None = Field(default=None, max_length=512,
                                  description="Optional focus question for the LLM")


class AnalyzeRequest(BaseModel):
    """POST /analyze — natural language question routed by Gamma Router (section 5)."""
    user_id: str = Field(..., min_length=1, max_length=128)
    question: str = Field(..., min_length=3, max_length=1024)
