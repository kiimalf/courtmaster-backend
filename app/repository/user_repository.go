package repository

import (
	"context"
	"errors"
	"fmt"

	"courtmaster-backend/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByIdentifier(ctx context.Context, identifier string) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]string, error)
}

type userPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userPostgresRepo{pool: pool}
}

func (r *userPostgresRepo) Create(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (username, name, email, password_hash, role_id)
	          VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, query, user.Username, user.Name, user.Email, user.PasswordHash, user.RoleID).
		Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return fmt.Errorf("userRepo.Create: %w", err)
	}
	return nil
}

func (r *userPostgresRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.FindByIdentifier(ctx, email)
}

func (r *userPostgresRepo) FindByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	query := `SELECT u.id, u.username, u.name, u.email, u.password_hash, u.role_id, r.name, u.created_at
	          FROM users u
	          JOIN roles r ON r.id = u.role_id
	          WHERE u.email = $1 OR u.username = $1`
	var u model.User
	err := r.pool.QueryRow(ctx, query, identifier).
		Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userPostgresRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	query := `SELECT u.id, u.username, u.name, u.email, u.password_hash, u.role_id, r.name, u.created_at
	          FROM users u
	          JOIN roles r ON r.id = u.role_id
	          WHERE u.id = $1`
	var u model.User
	err := r.pool.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userPostgresRepo) GetUserPermissions(ctx context.Context, userID int64) ([]string, error) {
	query := `SELECT p.name
	          FROM users u
	          JOIN role_permissions rp ON rp.role_id = u.role_id
	          JOIN permissions p ON p.id = rp.permission_id
	          WHERE u.id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}
