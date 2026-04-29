package cache

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/config"
)

type Runtime struct {
	Config  config.RedisConfig
	Timeout time.Duration
	Dial    func(context.Context, string, string) (net.Conn, error)
}

func NewRuntime(cfg config.RedisConfig) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := Verify(cfg); err != nil {
		return nil, err
	}
	return &Runtime{
		Config:  cfg,
		Timeout: 2 * time.Second,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 2 * time.Second}
			return dialer.DialContext(ctx, network, address)
		},
	}, nil
}

func Verify(cfg config.RedisConfig) error {
	if !cfg.Enabled {
		return nil
	}
	address := strings.TrimSpace(cfg.Addr)
	if address == "" {
		return fmt.Errorf("redis address is required")
	}
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func (r *Runtime) Ping(ctx context.Context) error {
	reply, err := r.command(ctx, "PING")
	if err != nil {
		return err
	}
	if reply != "PONG" {
		return fmt.Errorf("unexpected redis ping reply: %s", reply)
	}
	return nil
}

func (r *Runtime) Get(ctx context.Context, key string) (string, bool, error) {
	reply, err := r.command(ctx, "GET", r.prefixed(key))
	if err != nil {
		if strings.Contains(err.Error(), "redis nil") {
			return "", false, nil
		}
		return "", false, err
	}
	return reply, true, nil
}

func (r *Runtime) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	args := []string{"SET", r.prefixed(key), value}
	if ttl > 0 {
		args = append(args, "PX", strconv.FormatInt(ttl.Milliseconds(), 10))
	}
	reply, err := r.command(ctx, args...)
	if err != nil {
		return err
	}
	if reply != "OK" {
		return fmt.Errorf("unexpected redis set reply: %s", reply)
	}
	return nil
}

func (r *Runtime) Delete(ctx context.Context, key string) error {
	_, err := r.command(ctx, "DEL", r.prefixed(key))
	return err
}

func (r *Runtime) Increment(ctx context.Context, key string) (int64, error) {
	reply, err := r.command(ctx, "INCR", r.prefixed(key))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(reply), 10, 64)
}

func (r *Runtime) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	reply, err := r.command(ctx, "PEXPIRE", r.prefixed(key), strconv.FormatInt(ttl.Milliseconds(), 10))
	if err != nil {
		return err
	}
	if reply != "1" && reply != "OK" {
		return fmt.Errorf("unexpected redis expire reply: %s", reply)
	}
	return nil
}

func (r *Runtime) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, fmt.Errorf("lock ttl must be greater than zero")
	}
	reply, err := r.command(ctx, "SET", r.prefixed(key), token, "NX", "PX", strconv.FormatInt(ttl.Milliseconds(), 10))
	if err != nil {
		if strings.Contains(err.Error(), "redis nil") {
			return false, nil
		}
		return false, err
	}
	return reply == "OK", nil
}

func (r *Runtime) ReleaseLock(ctx context.Context, key, token string) error {
	prefixedKey := r.prefixed(key)
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("lock token is required")
	}
	_, err := r.command(ctx, "EVAL", "if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end return 0", "1", prefixedKey, token)
	return err
}

func (r *Runtime) prefixed(key string) string {
	if r == nil {
		return key
	}
	return strings.TrimSpace(r.Config.Prefix) + strings.TrimSpace(key)
}

func (r *Runtime) command(ctx context.Context, args ...string) (string, error) {
	if r == nil {
		return "", nil
	}
	dial := r.Dial
	if dial == nil {
		dialer := &net.Dialer{Timeout: r.Timeout}
		dial = dialer.DialContext
	}
	conn, err := dial(ctx, "tcp", r.Config.Addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(r.Timeout))

	if strings.TrimSpace(r.Config.Password) != "" {
		if _, err := r.run(conn, "AUTH", r.Config.Password); err != nil {
			return "", err
		}
	}
	if r.Config.DB > 0 {
		if _, err := r.run(conn, "SELECT", strconv.Itoa(r.Config.DB)); err != nil {
			return "", err
		}
	}
	return r.run(conn, args...)
}

func (r *Runtime) run(conn net.Conn, args ...string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, arg := range args {
		buf.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n")
		buf.WriteString(arg)
		buf.WriteString("\r\n")
	}
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return "", err
	}
	return parseReply(bufio.NewReader(conn))
}

func parseReply(reader *bufio.Reader) (string, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+', ':':
		return line, nil
	case '$':
		size, err := strconv.Atoi(line)
		if err != nil {
			return "", err
		}
		if size < 0 {
			return "", fmt.Errorf("redis nil")
		}
		payload := make([]byte, size+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return "", err
		}
		return string(payload[:size]), nil
	case '-':
		return "", fmt.Errorf("redis error: %s", line)
	default:
		return "", fmt.Errorf("unsupported redis reply prefix: %q", prefix)
	}
}
