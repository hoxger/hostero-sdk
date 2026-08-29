from __future__ import annotations

import httpx
import pytest

from hostero import (
    ApiError,
    AuthenticationError,
    ConfigurationError,
    ConflictError,
    ForbiddenError,
    Hostero,
    NotFoundError,
    RateLimitError,
    ValidationError,
)
from hostero._generated.operations import Operation


def test_request_uses_base_url_and_bearer_api_key() -> None:
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(200, json={})

    with Hostero(
        "hst_test_key",
        transport=httpx.MockTransport(handler),
    ) as client:
        client._request("GET", "/servers")

    assert len(requests) == 1
    assert str(requests[0].url) == "https://api.hostero.gg/v1/servers"
    assert requests[0].headers.get_list("authorization") == ["Bearer hst_test_key"]


def test_from_env_is_opt_in(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("HOSTERO_API_KEY", "hst_from_env")
    client = Hostero.from_env(
        transport=httpx.MockTransport(lambda request: httpx.Response(200))
    )

    try:
        assert "hst_from_env" not in repr(client)
    finally:
        client.close()


def test_from_env_requires_api_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("HOSTERO_API_KEY", raising=False)

    with pytest.raises(ConfigurationError, match="HOSTERO_API_KEY"):
        Hostero.from_env()


@pytest.mark.parametrize("api_key", ["", " ", "hst test", "\thst_test"])
def test_api_key_must_be_a_nonempty_token(api_key: str) -> None:
    with pytest.raises(ConfigurationError) as error:
        Hostero(api_key)

    if api_key.strip():
        assert api_key not in str(error.value)


def test_authorization_header_cannot_be_overridden() -> None:
    with Hostero(
        "hst_test_key",
        transport=httpx.MockTransport(lambda request: httpx.Response(200)),
    ) as client:
        with pytest.raises(ConfigurationError, match="Authorization"):
            client._request(
                "GET", "/servers", headers={"Authorization": "Bearer other"}
            )


@pytest.mark.parametrize(
    ("status_code", "error_type"),
    [
        (401, AuthenticationError),
        (403, ForbiddenError),
        (404, NotFoundError),
        (409, ConflictError),
        (422, ValidationError),
        (429, RateLimitError),
        (500, ApiError),
    ],
)
def test_maps_api_errors_without_leaking_api_key(
    status_code: int,
    error_type: type[ApiError],
) -> None:
    operation = Operation(
        operation_id="restart_game_server",
        method="POST",
        path="/servers/{server_id}/power/restart",
        required_permissions=("game_servers.power.restart",),
        target_kinds=("game_server",),
    )
    api_key = "hst_secret_key"

    with Hostero(
        api_key,
        transport=httpx.MockTransport(
            lambda request: httpx.Response(
                status_code,
                json={"code": "DENIED", "detail": f"no access for {api_key}"},
            )
        ),
    ) as client:
        with pytest.raises(error_type) as raised:
            client._request("POST", operation.path, operation=operation)

    error = raised.value
    assert api_key not in str(error)
    assert api_key not in repr(error.detail)
    if isinstance(error, ForbiddenError):
        assert error.required_permissions == ("game_servers.power.restart",)


def test_close_closes_connection_pool() -> None:
    client = Hostero(
        "hst_test_key",
        transport=httpx.MockTransport(lambda request: httpx.Response(200)),
    )

    client.close()

    with pytest.raises(RuntimeError, match="closed"):
        client._request("GET", "/servers")
