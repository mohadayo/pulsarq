import { app, checkService } from "../server";
import http from "http";

let server: http.Server;
const TEST_PORT = 4999;

beforeAll((done) => {
  server = app.listen(TEST_PORT, done);
});

afterAll((done) => {
  server.close(done);
});

describe("GET /health", () => {
  it("returns healthy status", async () => {
    const resp = await fetch(`http://localhost:${TEST_PORT}/health`);
    const data = (await resp.json()) as { service: string; status: string; timestamp: number };
    expect(resp.status).toBe(200);
    expect(data.service).toBe("dashboard");
    expect(data.status).toBe("healthy");
    expect(data.timestamp).toBeDefined();
  });
});

describe("GET /", () => {
  it("returns HTML dashboard page", async () => {
    const resp = await fetch(`http://localhost:${TEST_PORT}/`);
    const text = await resp.text();
    expect(resp.status).toBe(200);
    expect(text).toContain("PulsarQ Dashboard");
    expect(text).toContain("<!DOCTYPE html>");
  });
});

describe("GET /api/status", () => {
  it("returns service status array", async () => {
    const resp = await fetch(`http://localhost:${TEST_PORT}/api/status`);
    const data = (await resp.json()) as { services: Array<{ name: string; status: string }>; checked_at: string };
    expect(resp.status).toBe(200);
    expect(data.services).toHaveLength(2);
    expect(data.checked_at).toBeDefined();
    expect(data.services[0].name).toBe("api-gateway");
    expect(data.services[1].name).toBe("task-worker");
  });
});

describe("GET /api/tasks", () => {
  it("returns 503 when api gateway is down", async () => {
    const resp = await fetch(`http://localhost:${TEST_PORT}/api/tasks`);
    expect(resp.status).toBe(503);
    const data = (await resp.json()) as { error: string };
    expect(data.error).toBeDefined();
  });
});

describe("checkService", () => {
  it("returns down status for unreachable service", async () => {
    const result = await checkService("test-svc", "http://localhost:1");
    expect(result.name).toBe("test-svc");
    expect(result.status).toBe("down");
  });
});
