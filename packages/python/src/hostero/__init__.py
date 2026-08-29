"""Python SDK for the Hostero API."""

from ._client import DEFAULT_BASE_URL, Hostero
from ._errors import (
    ApiError,
    AuthenticationError,
    ConfigurationError,
    ConflictError,
    ForbiddenError,
    HosteroError,
    NotFoundError,
    RateLimitError,
    ValidationError,
)

__all__ = [
    "DEFAULT_BASE_URL",
    "ApiError",
    "AuthenticationError",
    "ConfigurationError",
    "ConflictError",
    "ForbiddenError",
    "Hostero",
    "HosteroError",
    "NotFoundError",
    "RateLimitError",
    "ValidationError",
]
