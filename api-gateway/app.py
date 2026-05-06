import json
import logging
import os
import time
import uuid

from flask import Flask, jsonify, request

app = Flask(__name__)

LOG_LEVEL = os.environ.get("LOG_LEVEL", "INFO").upper()
logging.basicConfig(
    level=getattr(logging, LOG_LEVEL, logging.INFO),
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("api-gateway")

REDIS_HOST = os.environ.get("REDIS_HOST", "localhost")
REDIS_PORT = int(os.environ.get("REDIS_PORT", "6379"))

_redis_client = None


def get_redis():
    global _redis_client
    if _redis_client is None:
        import redis

        _redis_client = redis.Redis(
            host=REDIS_HOST, port=REDIS_PORT, decode_responses=True
        )
    return _redis_client


@app.route("/health")
def health():
    status = {"service": "api-gateway", "status": "healthy", "timestamp": time.time()}
    try:
        get_redis().ping()
        status["redis"] = "connected"
    except Exception:
        status["redis"] = "disconnected"
    return jsonify(status)


@app.route("/api/tasks", methods=["POST"])
def create_task():
    data = request.get_json(silent=True)
    if not data or "name" not in data:
        logger.warning("Invalid task creation request: missing 'name'")
        return jsonify({"error": "Field 'name' is required"}), 400

    task_id = str(uuid.uuid4())
    task = {
        "id": task_id,
        "name": data["name"],
        "payload": data.get("payload", {}),
        "priority": data.get("priority", "normal"),
        "status": "queued",
        "created_at": time.time(),
    }

    try:
        r = get_redis()
        mapping = {k: json.dumps(v) if isinstance(v, dict) else str(v) for k, v in task.items()}
        r.hset(f"task:{task_id}", mapping=mapping)
        r.lpush("task_queue", task_id)
        logger.info("Task created: %s (priority=%s)", task_id, task["priority"])
    except Exception as e:
        logger.error("Failed to enqueue task: %s", e)
        return jsonify({"error": "Failed to enqueue task"}), 503

    return jsonify(task), 201


@app.route("/api/tasks/<task_id>", methods=["GET"])
def get_task(task_id):
    try:
        r = get_redis()
        raw = r.hgetall(f"task:{task_id}")
        if not raw:
            return jsonify({"error": "Task not found"}), 404
        task = {}
        for k, v in raw.items():
            try:
                task[k] = json.loads(v)
            except (json.JSONDecodeError, TypeError):
                task[k] = v
        return jsonify(task)
    except Exception as e:
        logger.error("Failed to fetch task %s: %s", task_id, e)
        return jsonify({"error": "Service unavailable"}), 503


@app.route("/api/tasks", methods=["GET"])
def list_tasks():
    try:
        r = get_redis()
        keys = r.keys("task:*")
        tasks = []
        for key in keys:
            raw = r.hgetall(key)
            task = {}
            for k, v in raw.items():
                try:
                    task[k] = json.loads(v)
                except (json.JSONDecodeError, TypeError):
                    task[k] = v
            tasks.append(task)
        tasks.sort(key=lambda t: float(t.get("created_at", 0)), reverse=True)
        return jsonify(tasks)
    except Exception as e:
        logger.error("Failed to list tasks: %s", e)
        return jsonify({"error": "Service unavailable"}), 503


if __name__ == "__main__":
    port = int(os.environ.get("API_PORT", "5000"))
    logger.info("Starting API Gateway on port %d", port)
    app.run(host="0.0.0.0", port=port)
