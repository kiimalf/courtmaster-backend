CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role_id INT NOT NULL REFERENCES roles(id),
    daily_booking_hours INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed User (username: password)
-- admin: AdminMaster2026
-- manager_central: ManagerPadel123
-- player_budi: BudiMainPadel2026
-- player_sari: SariBadminton2026
INSERT INTO users (username, name, email, password_hash, role_id) VALUES
    ('admin', 'Admin CourtMaster', 'admin@courtmaster.id', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 1),
    ('manager_central', 'Manager Central Arena', 'manager@courtmaster.id', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 2),
    ('player_budi', 'Budi Santoso', 'budi@gmail.com', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 3),
    ('player_sari', 'Sari Handayani', 'sari@gmail.com', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 3)
ON CONFLICT DO NOTHING;