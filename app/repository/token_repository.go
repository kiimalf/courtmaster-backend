package repository

import (
	"context"
	"errors"
	"time"

	"courtmaster-backend/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRepository interface {
	Save(ctx context.Context, token *model.RefreshToken) error
	FindActiveByHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, id int64) error
	RevokeAllUserTokens(ctx context.Context, userID int64) error
}

type tokenPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewTokenRepository(pool *pgxpool.Pool) TokenRepository {
	return &tokenPostgresRepo{pool: pool}
}

func (r *tokenPostgresRepo) Save(ctx context.Context, t *model.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
	          VALUES ($1, $2, $3) RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, t.UserID, t.TokenHash, t.ExpiresAt).Scan(&t.ID, &t.CreatedAt)
}

func (r *tokenPostgresRepo) FindActiveByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, revoked_at, expires_at, created_at
	          FROM refresh_tokens
	          WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`
	var t model.RefreshToken
	err := r.pool.QueryRow(ctx, query, hash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.RevokedAt, &t.ExpiresAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tokenPostgresRepo) Revoke(ctx context.Context, id int64) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *tokenPostgresRepo) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, time.Now(), userID)
	return err
}
