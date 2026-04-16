#
# READ-ONLY queries against Person 1's PostgreSQL ledger.
#
# Schema (migrations/001_init.sql):
#   journal_entries(id, transaction_id, account_id, debit_amount BIGINT, credit_amount BIGINT, created_at)
#   accounts(id, user_id, name TEXT, type: 'asset'|'liability'|'revenue'|'expense'|'equity', created_at)
#   transactions(id, user_id, source TEXT, external_reference, description, created_at)
#
# Amounts are BIGINT cents — divide by 100.0 to get USD floats.
# Double-entry conventions:
#   Revenue  accounts: credit_amount increases, debit_amount decreases (refunds)
#   Expense  accounts: debit_amount increases, credit_amount decreases
#   Asset    accounts: debit_amount increases balance
#   Liability accounts: credit_amount increases balance
from __future__ import annotations

from datetime import date
from typing import Any

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine

_GRAN = {"daily": "day", "weekly": "week", "monthly": "month"}


class LedgerRepository:
    """Async read-only queries against Person 1's ledger. All amounts returned in USD."""

    def __init__(self, engine: AsyncEngine) -> None:
        self.engine = engine

    async def get_metric_by_period(
        self,
        *,
        user_id: str,
        metric: str,
        start_date: date,
        end_date: date,
        granularity: str,
    ) -> list[dict[str, Any]]:
        """Returns [{"period": date, "value": float}, ...] ordered chronologically."""
        gran = _GRAN.get(granularity, "month")
        builders = {
            "revenue":    self._revenue,
            "refunds":    self._refunds,
            "expenses":   self._expenses,
            "net_income": self._net_income,
            "profit":     self._net_income,
            "balance":    self._balance,
        }
        sql = builders.get(metric, self._revenue)(gran)
        params = {"user_id": user_id, "start": start_date, "end": end_date}
        async with self.engine.connect() as conn:
            rows = await conn.execute(text(sql), params)
            return [{"period": r.period, "value": float(r.value)} for r in rows]

    async def get_total_for_period(
        self, *, user_id: str, start_date: date, end_date: date
    ) -> float:
        sql = """
            SELECT COALESCE(SUM(je.credit_amount - je.debit_amount), 0) / 100.0 AS total
            FROM journal_entries je
            JOIN transactions t ON t.id = je.transaction_id
            JOIN accounts     a ON a.id = je.account_id
            WHERE a.user_id = :user_id AND a.type = 'revenue'
              AND t.created_at >= :start AND t.created_at <= :end
        """
        async with self.engine.connect() as conn:
            row = (await conn.execute(text(sql), {"user_id": user_id, "start": start_date, "end": end_date})).first()
            return float(row.total) if row else 0.0

    # ── SQL builders ──────────────────────────────────────────────────────────

    @staticmethod
    def _p(gran: str) -> str:
        return f"DATE_TRUNC('{gran}', t.created_at)::date AS period"

    @staticmethod
    def _joins() -> str:
        return ("FROM journal_entries je\n"
                "JOIN transactions t ON t.id = je.transaction_id\n"
                "JOIN accounts     a ON a.id = je.account_id")

    def _revenue(self, gran: str) -> str:
        return f"""
            SELECT {self._p(gran)},
                   COALESCE(SUM(je.credit_amount - je.debit_amount), 0) / 100.0 AS value
            {self._joins()}
            WHERE a.user_id = :user_id AND a.type = 'revenue'
              AND t.created_at >= :start AND t.created_at <= :end
            GROUP BY period ORDER BY period
        """

    def _refunds(self, gran: str) -> str:
        return f"""
            SELECT {self._p(gran)},
                   COALESCE(SUM(je.debit_amount), 0) / 100.0 AS value
            {self._joins()}
            WHERE a.user_id = :user_id AND a.type = 'revenue' AND je.debit_amount > 0
              AND t.created_at >= :start AND t.created_at <= :end
            GROUP BY period ORDER BY period
        """

    def _expenses(self, gran: str) -> str:
        return f"""
            SELECT {self._p(gran)},
                   COALESCE(SUM(je.debit_amount - je.credit_amount), 0) / 100.0 AS value
            {self._joins()}
            WHERE a.user_id = :user_id AND a.type = 'expense'
              AND t.created_at >= :start AND t.created_at <= :end
            GROUP BY period ORDER BY period
        """

    def _net_income(self, gran: str) -> str:
        return f"""
            SELECT {self._p(gran)},
                   COALESCE(SUM(
                       CASE WHEN a.type = 'revenue' THEN  (je.credit_amount - je.debit_amount)
                            WHEN a.type = 'expense' THEN -(je.debit_amount  - je.credit_amount)
                            ELSE 0 END
                   ), 0) / 100.0 AS value
            {self._joins()}
            WHERE a.user_id = :user_id AND a.type IN ('revenue', 'expense')
              AND t.created_at >= :start AND t.created_at <= :end
            GROUP BY period ORDER BY period
        """

    def _balance(self, gran: str) -> str:
        return f"""
            SELECT {self._p(gran)},
                   COALESCE(SUM(
                       CASE WHEN a.type = 'asset'     THEN  (je.debit_amount  - je.credit_amount)
                            WHEN a.type = 'liability' THEN -(je.credit_amount - je.debit_amount)
                            ELSE 0 END
                   ), 0) / 100.0 AS value
            {self._joins()}
            WHERE a.user_id = :user_id AND a.type IN ('asset', 'liability')
              AND t.created_at >= :start AND t.created_at <= :end
            GROUP BY period ORDER BY period
        """
