import json
from unittest.mock import MagicMock, patch

import pytest

from app import app


@pytest.fixture
def client():
    app.config["TESTING"] = True
    with app.test_client() as c:
        yield c


def test_health_endpoint(client):
    with patch("app.get_redis") as mock_redis:
        mock_redis.return_value.ping.return_value = True
        resp = client.get("/health")
        data = resp.get_json()
        assert resp.status_code == 200
        assert data["service"] == "api-gateway"
        assert data["status"] == "healthy"
        assert data["redis"] == "connected"


def test_health_redis_down(client):
    with patch("app.get_redis") as mock_redis:
        mock_redis.return_value.ping.side_effect = Exception("down")
        resp = client.get("/health")
        data = resp.get_json()
        assert resp.status_code == 200
        assert data["redis"] == "disconnected"


def test_create_task_success(client):
    with patch("app.get_redis") as mock_redis:
        mock_r = MagicMock()
        mock_redis.return_value = mock_r
        resp = client.post(
            "/api/tasks",
            data=json.dumps({"name": "test-task", "payload": {"key": "val"}}),
            content_type="application/json",
        )
        data = resp.get_json()
        assert resp.status_code == 201
        assert data["name"] == "test-task"
        assert data["status"] == "queued"
        assert "id" in data
        mock_r.hset.assert_called_once()
        mock_r.lpush.assert_called_once()


def test_create_task_missing_name(client):
    resp = client.post(
        "/api/tasks",
        data=json.dumps({"payload": {}}),
        content_type="application/json",
    )
    assert resp.status_code == 400
    assert "name" in resp.get_json()["error"].lower()


def test_create_task_empty_body(client):
    resp = client.post("/api/tasks", content_type="application/json")
    assert resp.status_code == 400


def test_get_task_found(client):
    with patch("app.get_redis") as mock_redis:
        mock_r = MagicMock()
        mock_r.hgetall.return_value = {
            "id": "abc-123",
            "name": "my-task",
            "status": "queued",
            "created_at": "1700000000",
        }
        mock_redis.return_value = mock_r
        resp = client.get("/api/tasks/abc-123")
        data = resp.get_json()
        assert resp.status_code == 200
        assert data["name"] == "my-task"


def test_get_task_not_found(client):
    with patch("app.get_redis") as mock_redis:
        mock_r = MagicMock()
        mock_r.hgetall.return_value = {}
        mock_redis.return_value = mock_r
        resp = client.get("/api/tasks/nonexistent")
        assert resp.status_code == 404


def test_list_tasks(client):
    with patch("app.get_redis") as mock_redis:
        mock_r = MagicMock()
        mock_r.keys.return_value = ["task:1", "task:2"]
        mock_r.hgetall.side_effect = [
            {"id": "1", "name": "t1", "created_at": "1700000001"},
            {"id": "2", "name": "t2", "created_at": "1700000000"},
        ]
        mock_redis.return_value = mock_r
        resp = client.get("/api/tasks")
        data = resp.get_json()
        assert resp.status_code == 200
        assert len(data) == 2
        assert str(data[0]["id"]) == "1"


def test_create_task_redis_failure(client):
    with patch("app.get_redis") as mock_redis:
        mock_r = MagicMock()
        mock_r.hset.side_effect = Exception("Redis down")
        mock_redis.return_value = mock_r
        resp = client.post(
            "/api/tasks",
            data=json.dumps({"name": "fail-task"}),
            content_type="application/json",
        )
        assert resp.status_code == 503
