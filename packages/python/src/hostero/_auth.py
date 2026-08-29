"""HTTPX authentication for Hostero API keys."""

import httpx


class _ApiKeyAuth(httpx.Auth):
    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def auth_flow(self, request: httpx.Request):
        request.headers["Authorization"] = f"Bearer {self._api_key}"
        yield request
