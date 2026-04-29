package app

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"game-admin/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRedisRuntimePingSetGetDeleteLock(t *testing.T) {
	t.Parallel()

	store := map[string]string{}
	lockOwners := map[string]string{}
	var mu sync.Mutex

	runtime := &RedisRuntime{
		Config:  config.RedisConfig{Enabled: true, Prefix: "game-admin:"},
		Timeout: 2 * time.Second,
		Dial: func(_ context.Context, _, _ string) (net.Conn, error) {
			client, server := net.Pipe()
			go serveFakeRedisConn(server, &mu, store, lockOwners)
			return client, nil
		},
	}

	ctx := context.Background()
	require.NoError(t, runtime.Ping(ctx))
	require.NoError(t, runtime.Set(ctx, "cache:key", "value-1", time.Second))

	value, ok, err := runtime.Get(ctx, "cache:key")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "value-1", value)

	locked, err := runtime.AcquireLock(ctx, "lock:key", "token-1", time.Second)
	require.NoError(t, err)
	require.True(t, locked)

	locked, err = runtime.AcquireLock(ctx, "lock:key", "token-2", time.Second)
	require.NoError(t, err)
	require.False(t, locked)

	require.NoError(t, runtime.ReleaseLock(ctx, "lock:key", "token-1"))
	require.NoError(t, runtime.Delete(ctx, "cache:key"))

	_, ok, err = runtime.Get(ctx, "cache:key")
	require.NoError(t, err)
	require.False(t, ok)
}

func serveFakeRedisConn(conn net.Conn, mu *sync.Mutex, store map[string]string, lockOwners map[string]string) {
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
			_, _ = conn.Write([]byte("-ERR empty command\r\n"))
			return
		}
		switch strings.ToUpper(args[0]) {
		case "PING":
			_, _ = conn.Write([]byte("+PONG\r\n"))
		case "AUTH", "SELECT":
			_, _ = conn.Write([]byte("+OK\r\n"))
		case "SET":
			key, value := args[1], args[2]
			nx := false
			for _, arg := range args[3:] {
				if strings.EqualFold(arg, "NX") {
					nx = true
					break
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
		case "GET":
			mu.Lock()
			value, ok := store[args[1]]
			mu.Unlock()
			if !ok {
				_, _ = conn.Write([]byte("$-1\r\n"))
				continue
			}
			_, _ = conn.Write([]byte("$" + strconv.Itoa(len(value)) + "\r\n" + value + "\r\n"))
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
			_, _ = conn.Write([]byte("-ERR unsupported\r\n"))
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
		if sizeLine == "" || sizeLine[0] != '$' {
			return nil, io.ErrUnexpectedEOF
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
