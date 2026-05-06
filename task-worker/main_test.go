package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := loadConfig()
	if cfg.RedisHost != "localhost" {
		t.Errorf("expected RedisHost=localhost, got %s", cfg.RedisHost)
	}
	if cfg.RedisPort != "6379" {
		t.Errorf("expected RedisPort=6379, got %s", cfg.RedisPort)
	}
	if cfg.WorkerPort != "5001" {
		t.Errorf("expected WorkerPort=5001, got %s", cfg.WorkerPort)
	}
	if cfg.Concurrency != 4 {
		t.Errorf("expected Concurrency=4, got %d", cfg.Concurrency)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	t.Setenv("REDIS_HOST", "redis-server")
	t.Setenv("WORKER_PORT", "9090")
	cfg := loadConfig()
	if cfg.RedisHost != "redis-server" {
		t.Errorf("expected RedisHost=redis-server, got %s", cfg.RedisHost)
	}
	if cfg.WorkerPort != "9090" {
		t.Errorf("expected WorkerPort=9090, got %s", cfg.WorkerPort)
	}
}

func TestGetEnv(t *testing.T) {
	if v := getEnv("NONEXISTENT_VAR_12345", "default"); v != "default" {
		t.Errorf("expected default, got %s", v)
	}
	t.Setenv("TEST_VAR_12345", "value")
	if v := getEnv("TEST_VAR_12345", "default"); v != "value" {
		t.Errorf("expected value, got %s", v)
	}
}

func TestHealthHandler(t *testing.T) {
	worker := &TaskWorker{
		config:    loadConfig(),
		redis:     NewRedisClient("localhost", "1"),
		processed: 5,
		failed:    2,
		startTime: time.Now().Add(-1 * time.Hour),
	}

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	worker.healthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Service != "task-worker" {
		t.Errorf("expected service=task-worker, got %s", resp.Service)
	}
	if resp.Status != "healthy" {
		t.Errorf("expected status=healthy, got %s", resp.Status)
	}
	if resp.Processed != 5 {
		t.Errorf("expected processed=5, got %d", resp.Processed)
	}
	if resp.Failed != 2 {
		t.Errorf("expected failed=2, got %d", resp.Failed)
	}
	if resp.Redis != "disconnected" {
		t.Errorf("expected redis=disconnected, got %s", resp.Redis)
	}
}

func TestNewTaskWorker(t *testing.T) {
	cfg := Config{
		RedisHost:  "127.0.0.1",
		RedisPort:  "6379",
		WorkerPort: "5001",
	}
	w := NewTaskWorker(cfg)
	if w.processed != 0 {
		t.Errorf("expected processed=0, got %d", w.processed)
	}
	if w.failed != 0 {
		t.Errorf("expected failed=0, got %d", w.failed)
	}
	if w.redis == nil {
		t.Error("expected redis client to be initialized")
	}
}

func TestParseSimpleString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"$-1\r\n", ""},
		{"$5\r\nhello\r\n", "hello"},
		{"$0\r\n\r\n", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := parseSimpleString(tt.input)
		if got != tt.expected {
			t.Errorf("parseSimpleString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseHashResponse(t *testing.T) {
	resp := "*4\r\n$4\r\nname\r\n$4\r\ntest\r\n$6\r\nstatus\r\n$6\r\nqueued\r\n"
	result := parseHashResponse(resp)
	if result["name"] != "test" {
		t.Errorf("expected name=test, got %s", result["name"])
	}
	if result["status"] != "queued" {
		t.Errorf("expected status=queued, got %s", result["status"])
	}

	empty := parseHashResponse("")
	if len(empty) != 0 {
		t.Errorf("expected empty map, got %v", empty)
	}
}
