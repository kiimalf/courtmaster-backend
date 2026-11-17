package repository

import (
	"context"
	"errors"
	"fmt"

	"courtmaster-backend/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CourtRepository interface {
	WithTx(tx DBTX) CourtRepository
	Create(ctx context.Context, court *model.Court) error
	FindByID(ctx context.Context, id int64) (*model.Court, error)
	FindAll(ctx context.Context, sportType string) ([]model.Court, error)
	UpdateHourlyRateOptimistic(ctx context.Context, id int64, newRate float64, currentVersion int) error
	FindSlotsByCourtAndDate(ctx context.Context, courtID int64, date string) ([]model.CourtSlot, error)
}

type courtPostgresRepo struct {
	db DBTX
}

func NewCourtRepository(pool *pgxpool.Pool) CourtRepository {
	return &courtPostgresRepo{db: pool}
}

func (r *courtPostgresRepo) WithTx(tx DBTX) CourtRepository {
	return &courtPostgresRepo{db: tx}
}

func (r *courtPostgresRepo) Create(ctx context.Context, court *model.Court) error {
	query := `INSERT INTO courts (name, sport_type, hourly_rate, version, is_active)
	          VALUES ($1, $2, $3, 1, true) RETURNING id, created_at`
	err := r.db.QueryRow(ctx, query, court.Name, court.SportType, court.HourlyRate).Scan(&court.ID, &court.CreatedAt)
	if err != nil {
		return fmt.Errorf("courtRepo.Create: %w", err)
	}

	return nil
}

func (r *courtPostgresRepo) FindByID(ctx context.Context, id int64) (*model.Court, error) {
	query := `SELECT id, name, sport_type, hourly_rate, version, is_active, created_at FROM courts WHERE id = $1`

	var c model.Court
	err := r.db.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.SportType, &c.HourlyRate, &c.Version, &c.IsActive, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return &c, nil
}

func (r *courtPostgresRepo) FindAll(ctx context.Context, sportType string) ([]model.Court, error) {
	query := `SELECT id, name, sport_type, hourly_rate, version, is_active, created_at FROM courts WHERE is_active = true`
	args := []any{}

	if sportType != "" {
		query += ` AND sport_type = $1`
		args = append(args, sportType)
	}
	query += ` ORDER BY id ASC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Court
	for rows.Next() {
		var c model.Court
		if err := rows.Scan(&c.ID, &c.Name, &c.SportType, &c.HourlyRate, &c.Version, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (r *courtPostgresRepo) UpdateHourlyRateOptimistic(ctx context.Context, id int64, newRate float64, currentVersion int) error {
	query := `UPDATE courts 
	          SET hourly_rate = $1, version = version + 1 
	          WHERE id = $2 AND version = $3`
	tag, err := r.db.Exec(ctx, query, newRate, id, currentVersion)

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrConflict
	}

	return nil
}

func (r *courtPostgresRepo) FindSlotsByCourtAndDate(ctx context.Context, courtID int64, date string) ([]model.CourtSlot, error) {
	query := `SELECT id, court_id, date::text, start_time::text, end_time::text, is_available, booking_id, version 
	          FROM court_slots WHERE court_id = $1 AND date = $2 ORDER BY start_time ASC`
	rows, err := r.db.Query(ctx, query, courtID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.CourtSlot
	for rows.Next() {
		var s model.CourtSlot
		if err := rows.Scan(&s.ID, &s.CourtID, &s.Date, &s.StartTime, &s.EndTime, &s.IsAvailable, &s.BookingID, &s.Version); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
