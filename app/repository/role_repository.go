package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepository interface {
	LoadPermissions(ctx context.Context) (map[string][]string, error)
}

type rolePostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &rolePostgresRepository{pool: pool}
}

func (r *rolePostgresRepository) LoadPermissions(ctx context.Context) (map[string][]string, error) {
	query := `
		SELECT r.name, COALESCE(p.name, '')
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		ORDER BY r.name, p.name
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mengambil permission: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var role, perm string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, fmt.Errorf("membaca row permission: %w", err)
		}
		if _, ok := result[role]; !ok {
			result[role] = []string{}
		}
		if perm != "" {
			result[role] = append(result[role], perm)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca hasil query permission: %w", err)
	}
	return result, nil
}
