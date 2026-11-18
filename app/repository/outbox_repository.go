package repository

import (
	"context"
	"fmt"
	"time"

	"courtmaster-backend/app/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository interface {
	CreateTx(ctx context.Context, tx DBTX, event *model.OutboxEvent) error
	FetchUnpublished(ctx context.Context, limit int) ([]model.OutboxEvent, error)
	MarkPublished(ctx context.Context, id int64) error
	CountPending(ctx context.Context) (int64, error)
}

type outboxPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) OutboxRepository {
	return &outboxPostgresRepo{pool: pool}
}

// CreateTx mencatat event outbox di dalam transaksi ACID yang sama dengan booking
// (Transactional Outbox Pattern: tidak ada dual-write ke broker).
func (r *outboxPostgresRepo) CreateTx(ctx context.Context, tx DBTX, event *model.OutboxEvent) error {
	query := `INSERT INTO outbox (event_type, payload) VALUES ($1, $2) RETURNING id, created_at`
	if err := tx.QueryRow(ctx, query, event.EventType, event.Payload).Scan(&event.ID, &event.CreatedAt); err != nil {
		return fmt.Errorf("outboxRepo.CreateTx: %w", err)
	}
	return nil
}

// FetchUnpublished mengambil batch event yang belum dipublikasikan.
//
// Catatan desain: klausa `FOR UPDATE SKIP LOCKED` sengaja tidak dipakai di sini
// karena lock akan langsung dilepas saat statement selesai, jauh sebelum event
// berhasil di-enqueue ke Redis. Proteksi duplikasi yang benar dilakukan di sisi
// relay dengan asynq.TaskID("outbox-<id>") + idempotensi klaim di consumer.
func (r *outboxPostgresRepo) FetchUnpublished(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	query := `SELECT id, event_type, payload, created_at, published_at
	          FROM outbox
	          WHERE published_at IS NULL
	          ORDER BY id ASC
	          LIMIT $1`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("outboxRepo.FetchUnpublished: %w", err)
	}
	defer rows.Close()

	events := make([]model.OutboxEvent, 0, limit)
	for rows.Next() {
		var e model.OutboxEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.CreatedAt, &e.PublishedAt); err != nil {
			return nil, fmt.Errorf("outboxRepo.FetchUnpublished.scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outboxRepo.FetchUnpublished.rows: %w", err)
	}
	return events, nil
}

// MarkPublished bersifat idempotent: hanya mengisi published_at pada event yang
// masih NULL, sehingga aman dipanggil ulang oleh relay paralel.
func (r *outboxPostgresRepo) MarkPublished(ctx context.Context, id int64) error {
	query := `UPDATE outbox SET published_at = $1 WHERE id = $2 AND published_at IS NULL`
	if _, err := r.pool.Exec(ctx, query, time.Now(), id); err != nil {
		return fmt.Errorf("outboxRepo.MarkPublished: %w", err)
	}
	return nil
}

func (r *outboxPostgresRepo) CountPending(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM outbox WHERE published_at IS NULL`
	if err := r.pool.QueryRow(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("outboxRepo.CountPending: %w", err)
	}
	return count, nil
}
