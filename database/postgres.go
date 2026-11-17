package database

import (
	"context"
	"fmt"
	"time"

	"courtmaster-backend/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&pool_max_conns=%s",
		config.GetEnv("DB_USER", "courtmaster"),
		config.GetEnv("DB_PASSWORD", "secret"),
		config.GetEnv("DB_HOST", "localhost"),
		config.GetEnv("DB_PORT", "5432"),
		config.GetEnv("DB_NAME", "courtmaster_db"),
		config.GetEnv("DB_SSLMODE", "disable"),
		config.GetEnv("DB_MAX_CONNS", "10"),
	)

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("Konfigurasi database tidak valid: %w", err)
	}

	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("Gagal memuat pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Gagal terhubung ke database: %w", err)
	}

	return pool, nil
}
