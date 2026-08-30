from __future__ import annotations

import httpx

from hostero import Hostero, RedirectResponse, Upload
from hostero._generated.enums import GameServerStatus
from hostero._generated.models import (
    GameServerListItemResource,
    PaginatedResponseGameServerListItemResource,
)


def test_servers_list_and_power_operations() -> None:
    captured_requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        captured_requests.append(request)
        if request.url.path == "/v1/servers" and request.method == "GET":
            return httpx.Response(
                200,
                json={
                    "items": [
                        {
                            "id": "srv_123",
                            "short_id": "srv123",
                            "name": "My Minecraft Server",
                            "status": "running",
                            "node_name": "node-1",
                            "game_name": "Minecraft",
                            "limits": {
                                "cpu": 200,
                                "memory_mb": 4096,
                                "disk_mb": 10240,
                                "max_allocations": 2,
                                "max_backups": 3,
                                "max_databases": 1,
                            },
                            "primary_allocation": None,
                            "expires_at": None,
                            "scheduled_delete_at": None,
                            "permissions": None,
                            "extra_unknown_field_from_future_api": "should_be_ignored",
                        }
                    ],
                    "has_next": False,
                    "total": 1,
                    "limit": 20,
                    "offset": 0,
                },
            )
        if (
            request.url.path == "/v1/servers/srv_123/power/restart"
            and request.method == "POST"
        ):
            return httpx.Response(204)
        return httpx.Response(404)

    with Hostero("hst_test_token", transport=httpx.MockTransport(handler)) as client:
        # Test client.servers.list()
        res: PaginatedResponseGameServerListItemResource = client.servers.list(limit=20)
        assert res.total == 1
        assert len(res.items) == 1
        item: GameServerListItemResource = res.items[0]
        assert item.id == "srv_123"
        assert item.name == "My Minecraft Server"
        assert item.status == GameServerStatus.RUNNING

        # Test client.servers.power.restart(server_id)
        restart_res = client.servers.power.restart("srv_123")
        assert restart_res is None

    assert len(captured_requests) == 2
    assert captured_requests[0].url.query == b"limit=20"
    assert captured_requests[1].url.path == "/v1/servers/srv_123/power/restart"


def test_servers_files_contents_get_and_write() -> None:
    captured_requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        captured_requests.append(request)
        if "/files/contents" in request.url.path:
            if request.method == "GET":
                return httpx.Response(
                    200, json={"content": "server-port=25565\nmotd=Hostero"}
                )
            if request.method == "POST":
                return httpx.Response(204)
        return httpx.Response(404)

    with Hostero("hst_test_token", transport=httpx.MockTransport(handler)) as client:
        contents_res = client.servers.files.contents.get(
            "srv_456", file="/server.properties"
        )
        assert contents_res.content == "server-port=25565\nmotd=Hostero"

        write_res = client.servers.files.contents.write(
            "srv_456", file="/server.properties", content=b"new content"
        )
        assert write_res is None

    assert len(captured_requests) == 2
    assert captured_requests[0].url.path == "/v1/servers/srv_456/files/contents"
    assert b"file=%2Fserver.properties" in captured_requests[0].url.query
    assert captured_requests[1].content == b"new content"


def test_tickets_messages_attachments_create_upload() -> None:
    captured_requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        captured_requests.append(request)
        return httpx.Response(
            201,
            json={
                "id": "550e8400-e29b-41d4-a716-446655440000",
                "filename": "latest.log",
                "size": 1024,
                "content_type": "text/plain",
                "created_at": "2026-08-29T20:00:00Z",
            },
        )

    with Hostero("hst_test_token", transport=httpx.MockTransport(handler)) as client:
        upload = Upload.from_bytes(b"log data line 1\nline 2", "latest.log")
        attachment = client.tickets.messages.attachments.create(
            "11111111-1111-1111-1111-111111111111",
            "22222222-2222-2222-2222-222222222222",
            file=upload,
        )
        assert attachment.filename == "latest.log"
        assert attachment.size == 1024

    assert len(captured_requests) == 1
    req = captured_requests[0]
    assert (
        req.url.path
        == "/v1/tickets/11111111-1111-1111-1111-111111111111/messages/22222222-2222-2222-2222-222222222222/attachments"
    )
    assert "multipart/form-data" in req.headers.get("content-type", "")


def test_servers_backups_download_redirect_302() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            302,
            headers={
                "Location": "https://storage.example.com/backup-123.tar.gz?signature=xyz"
            },
        )

    with Hostero("hst_test_token", transport=httpx.MockTransport(handler)) as client:
        redirect = client.servers.backups.download(
            "srv_789",
            "33333333-3333-3333-3333-333333333333",
            component_id="44444444-4444-4444-4444-444444444444",
        )
        assert isinstance(redirect, RedirectResponse)
        assert (
            redirect.url
            == "https://storage.example.com/backup-123.tar.gz?signature=xyz"
        )
