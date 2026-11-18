CREATE TABLE IF NOT EXISTS bookings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    court_id BIGINT NOT NULL REFERENCES courts(id),
    booking_code VARCHAR(30) UNIQUE NOT NULL,
    date DATE NOT NULL,
    total_amount NUMERIC(12, 2) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'CONFIRMED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sparrings (
    id BIGSERIAL PRIMARY KEY,
    host_id BIGINT NOT NULL REFERENCES users(id),
    court_id BIGINT NOT NULL REFERENCES courts(id),
    title VARCHAR(150) NOT NULL,
    date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    max_participants INT NOT NULL,
    current_participants INT NOT NULL DEFAULT 1,
    price_per_person NUMERIC(12, 2) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sparring_members (
    id BIGSERIAL PRIMARY KEY,
    sparring_id BIGINT NOT NULL REFERENCES sparrings(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_sparring_user UNIQUE (sparring_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_bookings_user_date ON bookings(user_id, date);
CREATE INDEX IF NOT EXISTS idx_bookings_keyset ON bookings(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_sparrings_open ON sparrings(date, start_time) WHERE status = 'OPEN';
