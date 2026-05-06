package main

import (
	"fmt"
	"net"
	"time"
)

type RedisClient struct {
	host string
	port string
}

func NewRedisClient(host, port string) *RedisClient {
	return &RedisClient{host: host, port: port}
}

func (r *RedisClient) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", r.host+":"+r.port, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("redis connect failed: %w", err)
	}
	return conn, nil
}

func (r *RedisClient) sendCommand(conn net.Conn, args ...string) (string, error) {
	cmd := fmt.Sprintf("*%d\r\n", len(args))
	for _, arg := range args {
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}
	_, err := conn.Write([]byte(cmd))
	if err != nil {
		return "", err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func (r *RedisClient) Ping() error {
	conn, err := r.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := r.sendCommand(conn, "PING")
	if err != nil {
		return err
	}
	if len(resp) < 5 || resp[0:5] != "+PONG" {
		return fmt.Errorf("unexpected response: %s", resp)
	}
	return nil
}

func (r *RedisClient) DequeueTask() (string, error) {
	conn, err := r.dial()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	resp, err := r.sendCommand(conn, "RPOP", "task_queue")
	if err != nil {
		return "", err
	}
	return parseSimpleString(resp), nil
}

func (r *RedisClient) GetTask(taskID string) (map[string]string, error) {
	conn, err := r.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	resp, err := r.sendCommand(conn, "HGETALL", "task:"+taskID)
	if err != nil {
		return nil, err
	}
	return parseHashResponse(resp), nil
}

func (r *RedisClient) UpdateTaskStatus(taskID, status string) error {
	conn, err := r.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = r.sendCommand(conn, "HSET", "task:"+taskID, "status", status)
	return err
}

func parseSimpleString(resp string) string {
	if len(resp) == 0 || resp[0] == '$' && resp[1] == '-' {
		return ""
	}
	if resp[0] == '$' {
		i := 0
		for i < len(resp) && resp[i] != '\n' {
			i++
		}
		i++
		j := i
		for j < len(resp) && resp[j] != '\r' {
			j++
		}
		if j > i {
			return resp[i:j]
		}
	}
	return ""
}

func parseHashResponse(resp string) map[string]string {
	result := make(map[string]string)
	if len(resp) == 0 || resp[0] != '*' {
		return result
	}

	values := parseBulkStrings(resp)
	for i := 0; i+1 < len(values); i += 2 {
		result[values[i]] = values[i+1]
	}
	return result
}

func parseBulkStrings(resp string) []string {
	var results []string
	i := 0
	for i < len(resp) && resp[i] != '\n' {
		i++
	}
	i++

	for i < len(resp) {
		if resp[i] == '$' {
			i++
			for i < len(resp) && resp[i] != '\n' {
				i++
			}
			i++
			start := i
			for i < len(resp) && resp[i] != '\r' {
				i++
			}
			results = append(results, resp[start:i])
			i += 2
		} else {
			i++
		}
	}
	return results
}
