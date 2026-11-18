package repository

import (
	"context"
	"errors"
	"fmt"

	"courtmaster-backend/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepository interface {
	Claim(ctx context.Context, tx DBTX, n *model.Notification) (bool, error)
	Create(ctx context.Context, tx DBTX, n *model.Notification) error
	FindByUserID(ctx context.Context, userID int64) ([]model.Notification, error)
}

type notificationPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &notificationPostgresRepo{pool: pool}
}

// Claim menerapkan pola idempotensi atomik (Modul 11): INSERT ... ON CONFLICT
// (outbox_id) DO NOTHING. Nilai false berarti notifikasi untuk event tersebut
// sudah pernah tercatat, sehingga consumer boleh skip tanpa mengirim ulang.
func (r *notificationPostgresRepo) Claim(ctx context.Context, tx DBTX, n *model.Notification) (bool, error) {
	var id int64
	query := `INSERT INTO notifications (outbox_id, user_id, channel, subject, body)
	          VALUES ($1, $2, $3, $4, $5)
	          ON CONFLICT (outbox_id) DO NOTHING
	          RETURNING id`
	err := tx.QueryRow(ctx, query, n.OutboxID, n.UserID, n.Channel, n.Subject, n.Body).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // Notifikasi sudah pernah tercatat (idempotent skip)
	}
	if err != nil {
		return false, fmt.Errorf("notificationRepo.Claim: %w", err)
	}
	n.ID = id
	return true, nil
}

func (r *notificationPostgresRepo) Create(ctx context.Context, tx DBTX, n *model.Notification) error {
	query := `INSERT INTO notifications (outbox_id, user_id, channel, subject, body)
	          VALUES ($1, $2, $3, $4, $5) RETURNING id, sent_at`
	if err := tx.QueryRow(ctx, query, n.OutboxID, n.UserID, n.Channel, n.Subject, n.Body).
		Scan(&n.ID, &n.SentAt); err != nil {
		return fmt.Errorf("notificationRepo.Create: %w", err)
	}
	return nil
}

// FindByUserID mengembalikan notifikasi terbaru milik pengguna. Dibatasi 100
// baris agar respons tetap ringan (feed notifikasi bukan data arsip).
func (r *notificationPostgresRepo) FindByUserID(ctx context.Context, userID int64) ([]model.Notification, error) {
	query := `SELECT id, outbox_id, user_id, channel, subject, body, sent_at
	          FROM notifications
	          WHERE user_id = $1
	          ORDER BY sent_at DESC
	          LIMIT 100`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("notificationRepo.FindByUserID: %w", err)
	}
	defer rows.Close()

	list := make([]model.Notification, 0)
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.OutboxID, &n.UserID, &n.Channel, &n.Subject, &n.Body, &n.SentAt); err != nil {
			return nil, fmt.Errorf("notificationRepo.FindByUserID.scan: %w", err)
		}
		list = append(list, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notificationRepo.FindByUserID.rows: %w", err)
	}
	return list, nil
}
