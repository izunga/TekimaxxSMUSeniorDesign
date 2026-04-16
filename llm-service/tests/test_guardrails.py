from __future__ import annotations
import pytest
from app.services.guardrails import (
    GuardrailViolation, strip_pii, check_compliance,
    check_numeric_in_text, validate_llm_output,
)


class TestStripPii:
    def test_ssn_redacted(self):
        assert "123-45-6789" not in strip_pii("SSN: 123-45-6789")
        assert "[REDACTED]" in strip_pii("SSN: 123-45-6789")

    def test_email_redacted(self):
        assert "user@example.com" not in strip_pii("email: user@example.com")

    def test_visa_card_redacted(self):
        assert "4111111111111111" not in strip_pii("card: 4111111111111111")

    def test_stripe_key_redacted(self):
        assert "sk_live_abc123" not in strip_pii("key: sk_live_abc123")

    def test_clean_text_unchanged(self):
        text = "Revenue grew 12% last quarter."
        assert strip_pii(text) == text


class TestComplianceCheck:
    def test_guaranteed_return_raises(self):
        with pytest.raises(GuardrailViolation):
            check_compliance("This is a guaranteed return of 20%.")

    def test_risk_free_raises(self):
        with pytest.raises(GuardrailViolation):
            check_compliance("This strategy is completely risk-free.")

    def test_you_will_make_raises(self):
        with pytest.raises(GuardrailViolation):
            check_compliance("You will make a lot of money.")

    def test_clean_text_passes(self):
        check_compliance("Revenue grew 10% last quarter.")   # no exception

    def test_investment_advice_raises(self):
        with pytest.raises(GuardrailViolation):
            check_compliance("My investment advice is buy this now.")


class TestNumericInText:
    def test_normal_amount_no_raise(self):
        check_numeric_in_text("Revenue was $50,000 last month.")   # no exception

    def test_no_dollar_amounts_safe(self):
        check_numeric_in_text("Revenue grew ten percent.")   # no exception


class TestValidateLlmOutput:
    def test_clean_passes_and_returns_text(self):
        text = "Your revenue was $12,000 last month, showing a healthy upward trend."
        result = validate_llm_output(text)
        assert result == text

    def test_pii_stripped_not_raised(self):
        result = validate_llm_output("Contact user@example.com for help.")
        assert "user@example.com" not in result
        assert "[REDACTED]" in result

    def test_compliance_raises(self):
        with pytest.raises(GuardrailViolation):
            validate_llm_output("You will definitely earn a guaranteed return.")
