"""Exceptions raised by the Hostero Python SDK."""

from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from ._generated.operations import Operation


class HosteroError(Exception):
    """Base error for the Hostero Python SDK."""


class ConfigurationError(HosteroError):
    """The SDK client was configured with an invalid value."""


class ApiError(HosteroError):
    """The Hostero API rejected a request."""

    def __init__(
        self,
        *,
        status_code: int,
        code: str | None = None,
        detail: object | None = None,
        operation: Operation | None = None,
    ) -> None:
        self.status_code = status_code
        self.code = code
        self.detail = detail
        self.operation = operation

        parts: list[str] = [f"HTTP {status_code}"]
        if code is not None:
            parts.append(f"({code})")
        header = f"Hostero API request failed with {' '.join(parts)}"
        if detail is not None:
            if isinstance(detail, str):
                message = f"{header}: {detail}"
            else:
                message = f"{header}: {detail!r}"
        else:
            message = header
        super().__init__(message)


class AuthenticationError(ApiError):
    """The API key is missing, invalid, revoked, or expired."""


class ForbiddenError(ApiError):
    """The API denied the request without revealing the authorization cause."""

    @property
    def required_permissions(self) -> tuple[str, ...]:
        if self.operation is None:
            return ()
        return self.operation.required_permissions


class NotFoundError(ApiError):
    """The requested resource is unavailable to this API key."""


class ConflictError(ApiError):
    """The request conflicts with the resource's current state."""


class ValidationError(ApiError):
    """The API rejected request input."""


class RateLimitError(ApiError):
    """The API rate-limited the request."""


def error_from_response(
    *,
    status_code: int,
    payload: object | None,
    operation: Operation | None,
    secret: str,
) -> ApiError:
    code, detail = _error_fields(payload, secret)
    error_type: type[ApiError]
    match status_code:
        case 401:
            error_type = AuthenticationError
        case 403:
            error_type = ForbiddenError
        case 404:
            error_type = NotFoundError
        case 409:
            error_type = ConflictError
        case 422:
            error_type = ValidationError
        case 429:
            error_type = RateLimitError
        case _:
            error_type = ApiError
    return error_type(
        status_code=status_code,
        code=code,
        detail=detail,
        operation=operation,
    )


def _error_fields(
    payload: object | None, secret: str
) -> tuple[str | None, object | None]:
    if not isinstance(payload, Mapping):
        return None, None
    raw_code = payload.get("code")
    code = raw_code if isinstance(raw_code, str) else None
    return _redact(code, secret), _redact(payload.get("detail"), secret)


def _redact(value: Any, secret: str) -> Any:
    if isinstance(value, str):
        return value.replace(secret, "[REDACTED]")
    if isinstance(value, list):
        return [_redact(item, secret) for item in value]
    if isinstance(value, dict):
        return {key: _redact(item, secret) for key, item in value.items()}
    return value
