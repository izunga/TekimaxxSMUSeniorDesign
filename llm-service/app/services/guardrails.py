#
# Section 3 — output validation guardrails applied to every LLM response.
# Three checks: PII detection, compliance language, numeric sanity.
from __future__ import annotations

import logging
import re

logger = logging.getLogger(__name__)

_PII = [re.compile(p) for p in [
    r"\b\d{3}-\d{2}-\d{4}\b",                               # SSN
    r"\b4[0-9]{12}(?:[0-9]{3})?\b",                          # Visa
    r"\b5[1-5][0-9]{14}\b",                                  # MC
    r"\b3[47][0-9]{13}\b",                                   # Amex
    r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b",  # email
    r"\b(\+1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b",  # US phone
    r"\bsk_live_[A-Za-z0-9]+\b",                             # Stripe live key
    r"\bsk_test_[A-Za-z0-9]+\b",                             # Stripe test key
]]

_COMPLIANCE = [re.compile(p, re.IGNORECASE) for p in [
    r"\b(guaranteed?)\b.{0,30}\b(return|profit|gain|revenue|income|growth)\b",
    r"\b(will definitely|certainly will|100%\s+sure|no risk|risk.?free)\b",
    r"\b(invest(ment)? advice|buy this|sell this|hold this)\b",
    r"\b(promise[sd]?)\b.{0,20}\b(return|profit|gain|money)\b",
    r"\byou (will|are going to) (make|earn|profit|gain)\b",
]]

_DOLLAR_RE = re.compile(r"\$\s*([\d,]+(?:\.\d{1,2})?)")
_MAX = 1_000_000_000
_MIN = -1_000_000_000


class GuardrailViolation(Exception):
    """Raised when a response fails a guardrail check."""


def strip_pii(text: str) -> str:
    for p in _PII:
        if p.search(text):
            logger.warning("guardrail: PII detected — redacting (pattern=%s)", p.pattern)
            text = p.sub("[REDACTED]", text)
    return text


def check_compliance(text: str, context: str = "response") -> None:
    """Hard reject responses containing compliance-violating language."""
    for p in _COMPLIANCE:
        if p.search(text):
            logger.error("guardrail: compliance violation in %s — rejecting", context)
            raise GuardrailViolation(f"Compliance language detected in {context}")


def check_numeric_in_text(text: str, context: str = "response") -> None:
    """Log a warning if the LLM mentions implausible dollar amounts."""
    for m in _DOLLAR_RE.finditer(text):
        raw = m.group(1).replace(",", "")
        try:
            v = float(raw)
        except ValueError:
            continue
        if not (_MIN <= v <= _MAX):
            logger.warning("guardrail: implausible amount $%s in %s", raw, context)


def validate_llm_output(text: str, context: str = "response") -> str:
    """
    Full guardrail pipeline for LLM-generated text.
    Returns cleaned text or raises GuardrailViolation.
    """
    text = strip_pii(text)
    check_compliance(text, context)
    check_numeric_in_text(text, context)
    return text
