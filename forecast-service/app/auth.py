from __future__ import annotations

import secrets

from fastapi import Header, HTTPException, status

from app.config import Settings, get_settings


def require_user_id(
    x_tekimax_user_id: str = Header(..., alias="X-Tekimax-User-Id"),
    x_internal_service_token: str | None = Header(default=None, alias="X-Internal-Service-Token"),
) -> str:
    user_id = x_tekimax_user_id.strip()
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Missing authenticated user identity",
        )

    settings: Settings = get_settings()
    if settings.allow_insecure_user_header:
        return user_id

    expected = settings.internal_service_token.strip()
    presented = (x_internal_service_token or "").strip()
    if expected and secrets.compare_digest(presented, expected):
        return user_id

    raise HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail="Missing or invalid internal service token",
    )
