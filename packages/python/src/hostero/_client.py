"""Synchronous Hostero API client."""

from __future__ import annotations

import os
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any

import httpx

from ._auth import _ApiKeyAuth
from ._errors import ConfigurationError, error_from_response

if TYPE_CHECKING:
    from ._generated.operations import Operation

DEFAULT_BASE_URL = "https://api.hostero.gg/v1"
_API_KEY_ENVIRONMENT_VARIABLE = "HOSTERO_API_KEY"


class Hostero:
    """A synchronous client for Hostero's API-key automation API."""

    def __init__(
        self,
        api_key: str,
        *,
        base_url: str = DEFAULT_BASE_URL,
        timeout: httpx.Timeout | float | None = 30.0,
        transport: httpx.BaseTransport | None = None,
    ) -> None:
        self._api_key = _validate_api_key(api_key)
        self._base_url = _normalise_base_url(base_url)
        self._client = httpx.Client(
            auth=_ApiKeyAuth(self._api_key),
            base_url=self._base_url,
            follow_redirects=False,
            timeout=timeout,
            transport=transport,
        )

    @classmethod
    def from_env(
        cls,
        *,
        base_url: str = DEFAULT_BASE_URL,
        timeout: httpx.Timeout | float | None = 30.0,
        transport: httpx.BaseTransport | None = None,
    ) -> Hostero:
        api_key = os.environ.get(_API_KEY_ENVIRONMENT_VARIABLE)
        if api_key is None:
            raise ConfigurationError(
                f"{_API_KEY_ENVIRONMENT_VARIABLE} environment variable is required"
            )
        return cls(
            api_key,
            base_url=base_url,
            timeout=timeout,
            transport=transport,
        )

    @property
    def base_url(self) -> httpx.URL:
        """The normalized base URL used for API requests."""
        return self._base_url

    def close(self) -> None:
        """Close the underlying HTTP connection pool."""
        self._client.close()

    def __enter__(self) -> Hostero:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()

    def _request(
        self,
        method: str,
        path: str,
        *,
        operation: Operation | None = None,
        headers: Mapping[str, str] | None = None,
        **kwargs: Any,
    ) -> httpx.Response:
        _reject_authorization_header(headers)
        response = self._client.request(
            method,
            path.lstrip("/"),
            headers=headers,
            **kwargs,
        )
        if response.is_error:
            raise error_from_response(
                status_code=response.status_code,
                payload=_json_payload(response),
                operation=operation,
                secret=self._api_key,
            )
        return response


def _validate_api_key(api_key: str) -> str:
    if (
        not isinstance(api_key, str)
        or not api_key
        or any(character.isspace() for character in api_key)
    ):
        raise ConfigurationError("api_key must be a non-empty token without whitespace")
    return api_key


def _normalise_base_url(base_url: str) -> httpx.URL:
    try:
        url = httpx.URL(base_url)
    except httpx.InvalidURL as error:
        raise ConfigurationError("base_url must be a valid HTTP URL") from error
    if url.scheme not in {"http", "https"} or url.host is None:
        raise ConfigurationError("base_url must be an absolute HTTP URL")
    return httpx.URL(f"{str(url).rstrip('/')}/")


def _reject_authorization_header(headers: Mapping[str, str] | None) -> None:
    if headers is not None and any(name.lower() == "authorization" for name in headers):
        raise ConfigurationError("Authorization is managed by the Hostero client")


def _json_payload(response: httpx.Response) -> object | None:
    try:
        return response.json()
    except ValueError:
        return None
