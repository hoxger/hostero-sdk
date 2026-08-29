from __future__ import annotations

from hostero._generated.enums import (
    GameServerStatus,
    ServerPermission,
    TicketCategory,
    TicketPriority,
    TicketStatus,
)
from hostero._generated.models import (
    GameServerListItemResource,
    GameServerListLimitsResource,
    GameServerListPrimaryResource,
    TicketCreateRequest,
    TicketResource,
)


def test_model_from_dict_and_to_dict_roundtrip() -> None:
    data = {
        "id": "srv_abc",
        "short_id": "srvabc",
        "name": "Production Minecraft",
        "status": "running",
        "node_name": "eu-central-1",
        "game_name": "Minecraft Java",
        "limits": {
            "cpu": 400,
            "memory_mb": 8192,
            "disk_mb": 20480,
            "max_allocations": 5,
            "max_backups": 5,
            "max_databases": 2,
        },
        "primary_allocation": {
            "ip": "1.2.3.4",
            "port": 25565,
            "protocol": "both",
        },
        "expires_at": "2026-12-31T23:59:59Z",
        "scheduled_delete_at": None,
        "permissions": ["server.view", "power.restart"],
    }

    server = GameServerListItemResource._from_dict(data)
    assert server.id == "srv_abc"
    assert server.status == GameServerStatus.RUNNING
    assert isinstance(server.limits, GameServerListLimitsResource)
    assert server.limits.cpu == 400
    assert isinstance(server.primary_allocation, GameServerListPrimaryResource)
    assert server.primary_allocation.ip == "1.2.3.4"
    assert server.primary_allocation.port == 25565
    assert server.permissions == [
        ServerPermission.SERVER_VIEW,
        ServerPermission.POWER_RESTART,
    ]

    serialized = server._to_dict()
    assert serialized["id"] == "srv_abc"
    assert serialized["status"] == "running"
    assert serialized["limits"]["cpu"] == 400
    assert serialized["primary_allocation"]["ip"] == "1.2.3.4"
    assert serialized["permissions"] == ["server.view", "power.restart"]


def test_model_from_dict_ignores_unknown_future_fields() -> None:
    data = {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "category": "technical",
        "created_at": "2026-08-29T21:00:00Z",
        "creator_user_id": "usr_123",
        "creator_user_name": "Jakub",
        "department_id": None,
        "last_activity_at": "2026-08-29T21:05:00Z",
        "priority": "medium",
        "status": "open",
        "subject": "Help with server",
        "updated_at": "2026-08-29T21:05:00Z",
        # Extra unknown fields from future API version
        "ai_summary": "User needs help",
        "sla_tier": "gold",
        "tags": ["urgent", "vip"],
    }

    ticket = TicketResource._from_dict(data)
    assert ticket.id == "550e8400-e29b-41d4-a716-446655440000"
    assert ticket.subject == "Help with server"
    assert ticket.category == TicketCategory.TECHNICAL
    assert ticket.priority == TicketPriority.MEDIUM
    assert ticket.status == TicketStatus.OPEN


def test_request_model_to_dict() -> None:
    req = TicketCreateRequest(
        category=TicketCategory.TECHNICAL,
        priority=TicketPriority.HIGH,
        subject="New ticket",
    )
    payload = req._to_dict()
    assert payload["category"] == "technical"
    assert payload["priority"] == "high"
    assert payload["subject"] == "New ticket"
