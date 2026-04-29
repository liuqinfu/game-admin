package app

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-admin/backend/internal/config"
	"game-admin/backend/internal/eventbus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SystemInfo struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"size:128;not null;uniqueIndex"`
	CreatedAt int64  `gorm:"autoCreateTime:milli"`
	UpdatedAt int64  `gorm:"autoUpdateTime:milli"`
}

func OpenSQLite(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database dsn is required")
	}

	if err := ensureSQLiteDir(dsn); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	return db, nil
}

func OpenMySQL(dsn string) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("database dsn is required")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open mysql database: %w", err)
	}

	return db, nil
}

func OpenDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "", "sqlite":
		return OpenSQLite(cfg.DSN)
	case "mysql":
		return OpenMySQL(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func ensureSQLiteDir(dsn string) error {
	if dsn == ":memory:" {
		return nil
	}

	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}

func VerifyRedis(cfg config.RedisConfig) error {
	if !cfg.Enabled {
		return nil
	}
	return verifyTCPAddress(cfg.Addr)
}

func VerifyMQ(cfg config.MQConfig) error {
	if !cfg.Enabled {
		return nil
	}
	transport, err := eventbus.NewTransport(cfg, eventbus.TransportOptions{})
	if err != nil {
		if errors.Is(err, eventbus.ErrUnsupportedMQDriver) {
			return err
		}
	} else if transport != nil {
		_ = transport.Close()
	}

	address := cfg.URL
	if strings.Contains(address, "://") {
		parts := strings.SplitN(address, "://", 2)
		address = parts[1]
	}
	if strings.Contains(address, "@") {
		segments := strings.SplitN(address, "@", 2)
		address = segments[1]
	}
	address = strings.TrimSuffix(address, "/")
	return verifyTCPAddress(address)
}

func VerifyHTTPHealth(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target is required")
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(target, "/") + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

func verifyTCPAddress(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("address is required")
	}

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
