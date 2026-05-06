package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	RedisHost   string
	RedisPort   string
	WorkerPort  string
	Concurrency int
	LogLevel    string
}

func loadConfig() Config {
	return Config{
		RedisHost:   getEnv("REDIS_HOST", "localhost"),
		RedisPort:   getEnv("REDIS_PORT", "6379"),
		WorkerPort:  getEnv("WORKER_PORT", "5001"),
		Concurrency: 4,
		LogLevel:    getEnv("LOG_LEVEL", "INFO"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type TaskWorker struct {
	config    Config
	redis     *RedisClient
	processed int
	failed    int
	startTime time.Time
}

func NewTaskWorker(cfg Config) *TaskWorker {
	return &TaskWorker{
		config:    cfg,
		redis:     NewRedisClient(cfg.RedisHost, cfg.RedisPort),
		startTime: time.Now(),
	}
}

type HealthResponse struct {
	Service   string  `json:"service"`
	Status    string  `json:"status"`
	Timestamp float64 `json:"timestamp"`
	Redis     string  `json:"redis"`
	Processed int     `json:"processed"`
	Failed    int     `json:"failed"`
	Uptime    string  `json:"uptime"`
}

func (w *TaskWorker) healthHandler(rw http.ResponseWriter, r *http.Request) {
	redisStatus := "connected"
	if err := w.redis.Ping(); err != nil {
		redisStatus = "disconnected"
	}

	resp := HealthResponse{
		Service:   "task-worker",
		Status:    "healthy",
		Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
		Redis:     redisStatus,
		Processed: w.processed,
		Failed:    w.failed,
		Uptime:    time.Since(w.startTime).String(),
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

func (w *TaskWorker) processTask(taskID string) error {
	log.Printf("[INFO] Processing task: %s", taskID)

	taskData, err := w.redis.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("failed to fetch task %s: %w", taskID, err)
	}

	if err := w.redis.UpdateTaskStatus(taskID, "processing"); err != nil {
		log.Printf("[WARN] Could not update status for %s: %v", taskID, err)
	}

	time.Sleep(100 * time.Millisecond)

	name, _ := taskData["name"]
	log.Printf("[INFO] Task %s (%s) completed successfully", taskID, name)

	if err := w.redis.UpdateTaskStatus(taskID, "completed"); err != nil {
		return fmt.Errorf("failed to mark task %s as completed: %w", taskID, err)
	}

	return nil
}

func (w *TaskWorker) run(ctx context.Context) {
	log.Printf("[INFO] Task worker started (concurrency=%d)", w.config.Concurrency)
	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Worker shutting down")
			return
		default:
			taskID, err := w.redis.DequeueTask()
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			if taskID == "" {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := w.processTask(taskID); err != nil {
				log.Printf("[ERROR] Task %s failed: %v", taskID, err)
				w.failed++
			} else {
				w.processed++
			}
		}
	}
}

func main() {
	cfg := loadConfig()
	worker := NewTaskWorker(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", worker.healthHandler)

	server := &http.Server{
		Addr:    ":" + cfg.WorkerPort,
		Handler: mux,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.run(ctx)

	go func() {
		log.Printf("[INFO] Health endpoint listening on :%s", cfg.WorkerPort)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[INFO] Received shutdown signal")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	log.Printf("[INFO] Worker stopped. Processed: %d, Failed: %d", worker.processed, worker.failed)
}
