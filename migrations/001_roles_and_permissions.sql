CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INT REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

INSERT INTO roles (id, name) VALUES 
    (1, 'admin'), 
    (2, 'manager'), 
    (3, 'player') 
ON CONFLICT DO NOTHING;

INSERT INTO permissions (name) VALUES
    ('court:create'), ('court:manage'),
    ('booking:create'), ('booking:read'), ('booking:update'), ('booking:export'),
    ('sparring:create'), ('sparring:join'),
    ('review:create'), ('review:moderate'),
    ('queue:monitor'), ('outbox:manage')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 1, id FROM permissions
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 2, id FROM permissions WHERE name IN ('court:create', 'court:manage', 'review:moderate', 'booking:read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 3, id FROM permissions WHERE name IN ('booking:create', 'booking:read', 'booking:update', 'booking:export', 'sparring:create', 'sparring:join', 'review:create')
ON CONFLICT DO NOTHING;