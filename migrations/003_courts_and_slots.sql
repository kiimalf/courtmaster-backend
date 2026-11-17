CREATE TABLE IF NOT EXISTS courts (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    sport_type VARCHAR(30) NOT NULL,
    hourly_rate NUMERIC(12, 2) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS court_slots (
    id BIGSERIAL PRIMARY KEY,
    court_id BIGINT NOT NULL REFERENCES courts(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    booking_id BIGINT,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_court_slot UNIQUE (court_id, date, start_time)
);

CREATE INDEX IF NOT EXISTS idx_slots_court_date ON court_slots(court_id, date);

INSERT INTO courts (id, name, sport_type, hourly_rate, version, is_active) VALUES
    (1, 'Lapangan Padel A (Vinyl Pro)', 'padel', 150000.00, 1, true),
    (2, 'Lapangan Badminton 1 (Karpet Yonex)', 'badminton', 75000.00, 1, true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO court_slots (court_id, date, start_time, end_time, is_available) VALUES
    (1, CURRENT_DATE, '08:00:00', '09:00:00', true),
    (1, CURRENT_DATE, '09:00:00', '10:00:00', true),
    (1, CURRENT_DATE, '10:00:00', '11:00:00', true),
    (1, CURRENT_DATE, '11:00:00', '12:00:00', true),
    (1, CURRENT_DATE + INTERVAL '1 day', '08:00:00', '09:00:00', true),
    (1, CURRENT_DATE + INTERVAL '1 day', '09:00:00', '10:00:00', true),
    (2, CURRENT_DATE, '08:00:00', '09:00:00', true),
    (2, CURRENT_DATE, '09:00:00', '10:00:00', true)
ON CONFLICT DO NOTHING;