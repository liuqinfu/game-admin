package worker

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
	gametrics "game-admin/backend/internal/metrics"
	"github.com/stretchr/testify/require"
)

func TestRunWithConsumerLockSkipsWhenHeld(t *testing.T) {
	t.Parallel()

	store := map[string]string{}
	lockOwners := map[string]string{}
	var mu sync.Mutex
	runtime := &app.RedisRuntime{
		Config:  config.RedisConfig{Enabled: true, Prefix: "game-admin:"},
		Timeout: 2 * time.Second,
		Dial: func(_ context.Context, _, _ string) (net.Conn, error) {
			client, server := net.Pipe()
			go serveLockRedisConn(server, &mu, store, lockOwners)
			return client, nil
		},
	}

	ctx := context.Background()
	lockToken := "holder"
	locked, err := runtime.AcquireLock(ctx, "worker-lock:test:consume", lockToken, time.Second)
	require.NoError(t, err)
	require.True(t, locked)

	calls := 0
	runWithConsumerLock(ctx, runtime, consumerLockConfig{
		ConsumerName: "test",
		WorkerName:   "worker-2",
		TTL:          time.Second,
		Stage:        "consume",
	}, func() {
		calls++
	})
	require.Equal(t, 0, calls)

	testLockKey := "game-admin:worker-lock:test:consume"
	lockOwners[testLockKey] = lockToken
	require.NoError(t, runtime.ReleaseLock(ctx, "worker-lock:test:consume", "wrong-token"))
	require.NoError(t, runtime.ReleaseLock(ctx, "worker-lock:test:consume", lockToken))
	delete(lockOwners, testLockKey)

	runWithConsumerLock(ctx, runtime, consumerLockConfig{
		ConsumerName: "test",
		WorkerName:   "worker-2",
		TTL:          time.Second,
		Stage:        "consume",
	}, func() {
		calls++
	})
	require.Equal(t, 1, calls)
}

func TestWorkerMetricsEndpointExposesPrometheus(t *testing.T) {
	t.Parallel()

	metricsRuntime := gametrics.NewRuntime("notification-service", "worker")
	metricsRuntime.RecordWorkerPoll("notification-service", "db", "ok")
	metricsRuntime.RecordWorkerDelivery("notification-service", "db", "succeeded")
	metricsRuntime.RecordWorkerLock("notification-service", "consume", "ok")
	healthRuntime := app.NewHealthRuntime(app.HealthStore{
		Service:      "notification-service",
		Version:      "dev",
		Dependencies: []app.DependencyHealth{{Name: "database", Status: "ok"}},
	})
	handler := newRuntimeMux(healthRuntime, RuntimeOptions{
		ConsumerName: "notification-service",
		Metrics:      metricsRuntime,
	})

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyResp := httptest.NewRecorder()
	handler.ServeHTTP(readyResp, readyReq)
	require.Equal(t, http.StatusOK, readyResp.Code)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_http_requests_total")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_worker_polls_total")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_worker_deliveries_total")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_worker_lock_acquisitions_total")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_ready_status")
}

func serveLockRedisConn(conn net.Conn, mu *sync.Mutex, store map[string]string, lockOwners map[string]string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readRESPArray(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_, _ = conn.Write([]byte("-ERR bad request\r\n"))
			return
		}
		if len(args) == 0 {
			return
		}
		switch strings.ToUpper(args[0]) {
		case "SET":
			key, value := args[1], args[2]
			nx := false
			for _, arg := range args[3:] {
				if strings.EqualFold(arg, "NX") {
					nx = true
				}
			}
			mu.Lock()
			if nx {
				if owner, ok := lockOwners[key]; ok && owner != value {
					store[key] = owner
					mu.Unlock()
					_, _ = conn.Write([]byte("$-1\r\n"))
					continue
				}
				if _, ok := store[key]; ok {
					mu.Unlock()
					_, _ = conn.Write([]byte("$-1\r\n"))
					continue
				}
			}
			store[key] = value
			lockOwners[key] = value
			mu.Unlock()
			_, _ = conn.Write([]byte("+OK\r\n"))
		case "DEL":
			mu.Lock()
			delete(store, args[1])
			delete(lockOwners, args[1])
			mu.Unlock()
			_, _ = conn.Write([]byte(":1\r\n"))
		case "EVAL":
			key := args[3]
			token := args[4]
			mu.Lock()
			current := lockOwners[key]
			if current == token || current == "" {
				delete(store, key)
				delete(lockOwners, key)
				mu.Unlock()
				_, _ = conn.Write([]byte(":1\r\n"))
				continue
			}
			mu.Unlock()
			_, _ = conn.Write([]byte(":0\r\n"))
		default:
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
	}
}

func readRESPArray(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if line == "" || line[0] != '*' {
		return nil, io.ErrUnexpectedEOF
	}
	count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		sizeLine, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(sizeLine[1:]))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
}
