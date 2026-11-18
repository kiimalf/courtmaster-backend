-- users: Inti identitas seluruh sistem (memuat username, email, dan daily_booking_hours sesuai SOP & Modul 10)
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

-- Seed Akun Default (Password Bcrypt cost 12):
-- admin           : AdminMaster2026
-- manager_central : ManagerPadel123
-- player_budi     : BudiMainPadel2026
-- player_sari     : SariBadminton2026
INSERT INTO users (username, name, email, password_hash, role_id) VALUES
    ('admin', 'Admin CourtMaster', 'admin@courtmaster.id', '$2a$12$5mMmITBUKcAdQyOw.UEPt.j1xuU3bvXpm.bCm6yzCOiprqXzF2RL6', 1),
    ('manager_central', 'Manager Central Arena', 'manager@courtmaster.id', '$2a$12$mb77Oakj2xpC3aAJUNRVveWAvtRRe8hG1XWRu9a/XPh46nIGdRsAO', 2),
    ('player_budi', 'Budi Santoso', 'budi@gmail.com', '$2a$12$y.Sb0P5vUZpASzJOs58jq.nEoCYQaEgoyHeLY4a8DLSY12rvsyzsC', 3),
    ('player_sari', 'Sari Handayani', 'sari@gmail.com', '$2a$12$3nldoHB7wJSuv5XkGuUHMOdwguo8U2uvlx5KNTZPKcFHVhH/s15yC', 3)
ON CONFLICT (username) DO NOTHING;
