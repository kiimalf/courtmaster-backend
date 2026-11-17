# PANDUAN PENGERJAAN — ANGGOTA 3
## Peran: Transactions, Concurrency, Keyset Pagination & Async Worker Lead
### Sistem: CourtMaster (Smart Sports Arena & Community Matchmaking)

---

## 1. Ikhtisar Tugas & Tanggung Jawab

Sebagai **Anggota 3**, Anda bertanggung jawab atas transaksi finansial/pemesanan lapangan dan pemrosesan asinkron yang dipecah menjadi **2 Pull Request (PR)** terpisah:
1. **PR 1 (`feat/booking-reservation`) — Transaksi Booking Lapangan, Keyset Pagination & Ekspor CSV (Modul 7, 9)**:
   - DDL tabel `bookings`, `sparrings`, dan `sparring_members`.
   - Transaksi ACID multi-tabel (`bookings` dan `court_slots`) menggunakan `TxManager.WithTx` dengan auto-retry deadlock (`40001`/`40P01`).
   - Pencegahan bentrok jam (*double booking*) dan validasi kuota maksimal booking 4 jam per user per hari.
   - *Keyset/Cursor Pagination* berbasis Base64 (`helper.EncodeCursor` & `helper.DecodeCursor`) untuk riwayat booking volume tinggi.
   - *Content Negotiation* header `Accept: text/csv` vs `application/json` untuk ekspor riwayat pemesanan RFC 4180.
   - Terdiri dari **Micro-Commit 1 s.d. 5**.
2. **PR 2 (`feat/outbox-worker`) — Transactional Outbox Pattern & Background Worker Asynq (Modul 11, 12)**:
   - DDL tabel `outbox` (dengan *partial index* `WHERE published_at IS NULL`) dan `notifications` (`UNIQUE (outbox_id)`).
   - Pencatatan event outbox ke tabel `outbox` di dalam transaksi database yang sama dengan booking (*Zero Message Loss*).
   - Relay Asynq dengan *Idempotency Key* (`asynq.TaskID("outbox-<id>")`) dan `asynq.Retention(24*time.Hour)`.
   - Background Worker konsumen Asynq yang memproses pengiriman notifikasi/email dengan proteksi idempotensi.
   - Dual-Process runner CLI (`-role=api`, `-role=worker`, atau `-role=all`).
   - Terdiri dari **Micro-Commit 6 s.d. 10**.

---

## 2. Aturan Emas Pengerjaan Bebas Konflik (Wajib Dibaca!)

1. **Prinsip Foundation Freeze**:
   - Seluruh dependensi (`go.mod`), fungsi helper (`cursor.go`, `negotiate.go`, `context.go`), dan tabel `users` **sudah disediakan oleh Anggota 1 di branch `develop`**.
   - **Tabel `users` sudah ada di database** dari migrasi `002`. Sedangkan tabel `courts` (`003_courts_and_slots.sql`) dan interface `CourtRepository` dikerjakan oleh Anggota 1 pada PR #1 (`feat/court-management`) dan akan otomatis masuk saat sinkronisasi `git merge origin/develop`.
   - **DILARANG menjalankan `go get` atau mengedit folder `helper/`**.
2. **Pola Integrasi Rute (Opsi B)**:
   - **DILARANG mengedit file `route/route.go` atau `main.go`** di branch Anda.
   - Uji seluruh aturan transaksi secara mandiri menggunakan **Unit Test (`booking_rules_test.go`)**.
   - Cantumkan daftar endpoint Anda di deskripsi Pull Request agar dihubungkan (*wired*) oleh Git Master (Anggota 1) saat integrasi di Tahap 2.

---

## 3. Eksekusi PR 1: `feat/booking-reservation` (Booking ACID & Keyset Pagination)

> [!NOTE]
> **Prasyarat Dependensi Court (DDL 003 & CourtRepository):**
> Micro-Commit 1 s.d. 4 (DDL, Model, BookingRepository, Unit Test) dapat langsung dikerjakan setelah Tahap 0. Untuk Micro-Commit 5 (`BookingService`), pastikan PR #1 (`feat/court-management`) milik Anggota 1 sudah di-merge ke branch `develop` (lakukan `git fetch origin develop && git merge origin/develop` di branch Anda) agar skema DDL `003_courts_and_slots.sql`, model `Court`, dan interface `repository.CourtRepository` sudah tersedia di repositori lokal Anda.

### Persiapan Branch PR 1:
```bash
git checkout develop
git pull origin develop
git checkout -b feat/booking-reservation
```

---

### Micro-Commit 1: DDL Migrasi Booking & Sparring

Tabel `users` sudah tersedia dari migrasi `002`, dan tabel `courts` akan masuk dari migrasi `003` saat sinkronisasi PR #1. Buat file **`migrations/005_bookings_and_sparrings.sql`**:

```sql
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
```

```bash
git add migrations/005_bookings_and_sparrings.sql
git commit -m "feat(booking): create bookings, sparrings, and sparring_members tables with keyset index"
```

---

### Micro-Commit 2: Model Entitas & Request/Response DTOs Booking & Sparring

Lengkapi file **`app/model/booking.go`** (entitas dasar `Booking` sudah diinisialisasi di Tahap 0 oleh Anggota 1, sekarang tambahkan DTO request dan entitas Sparring):

```go
package model

import "time"

type Booking struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	CourtID     int64     `json:"court_id"`
	BookingCode string    `json:"booking_code"`
	Date        string    `json:"date"`
	TotalAmount float64   `json:"total_amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateBookingRequest struct {
	CourtID int64   `json:"court_id" validate:"required,gt=0"`
	Date    string  `json:"date" validate:"required,datetime=2006-01-02"`
	SlotIDs []int64 `json:"slot_ids" validate:"required,min=1,max=4"`
}

type SwapBookingRequest struct {
	BookingID int64  `json:"booking_id" validate:"required,gt=0"`
	CourtID   int64  `json:"court_id" validate:"required,gt=0"`
	Date      string `json:"date" validate:"required,datetime=2006-01-02"`
	OldSlotID int64  `json:"old_slot_id" validate:"required,gt=0"`
	NewSlotID int64  `json:"new_slot_id" validate:"required,gt=0"`
}

type Sparring struct {
	ID                  int64     `json:"id"`
	HostID              int64     `json:"host_id"`
	CourtID             int64     `json:"court_id"`
	Title               string    `json:"title"`
	Date                string    `json:"date"`
	StartTime           string    `json:"start_time"`
	EndTime             string    `json:"end_time"`
	MaxParticipants     int       `json:"max_participants"`
	CurrentParticipants int       `json:"current_participants"`
	PricePerPerson      float64   `json:"price_per_person"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
}

type CreateSparringRequest struct {
	CourtID         int64   `json:"court_id" validate:"required,gt=0"`
	Title           string  `json:"title" validate:"required,min=5,max=150"`
	Date            string  `json:"date" validate:"required,datetime=2006-01-02"`
	StartTime       string  `json:"start_time" validate:"required"`
	EndTime         string  `json:"end_time" validate:"required"`
	MaxParticipants int     `json:"max_participants" validate:"required,min=2,max=20"`
	PricePerPerson  float64 `json:"price_per_person" validate:"required,gte=0"`
}
```

```bash
git add app/model/booking.go
git commit -m "feat(booking): define Booking, Sparring entities and reservation request DTOs"
```

---

### Micro-Commit 3: Repository Booking, Slot Locking & Sparring (PostgreSQL)

Buat file **`app/repository/booking_repository.go`**:

```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"courtmaster-backend/app/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository interface {
	Create(ctx context.Context, tx DBTX, booking *model.Booking) error
	FindByID(ctx context.Context, tx DBTX, id int64) (*model.Booking, error)
	LockAndAssignSlots(ctx context.Context, tx DBTX, courtID int64, date string, slotIDs []int64, bookingID int64) error
	SwapSlots(ctx context.Context, tx DBTX, bookingID, oldSlotID, newSlotID int64) error
	CountUserHoursByDate(ctx context.Context, tx DBTX, userID int64, date string) (int, error)
	FindBookingsKeyset(ctx context.Context, userID int64, limit int, cursor *model.Cursor) ([]model.Booking, error)
	HasUserBookedCourt(ctx context.Context, userID, courtID int64) (bool, error)

	// Sparring Methods (SOP Bagian 2 & 3)
	CreateSparring(ctx context.Context, tx DBTX, s *model.Sparring) error
	FindAllOpenSparrings(ctx context.Context) ([]model.Sparring, error)
	FindSparringByID(ctx context.Context, id int64) (*model.Sparring, error)
	JoinSparringAtomic(ctx context.Context, tx DBTX, sparringID, userID int64) error
}

type bookingPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewBookingRepository(pool *pgxpool.Pool) BookingRepository {
	return &bookingPostgresRepo{pool: pool}
}

func (r *bookingPostgresRepo) Create(ctx context.Context, tx DBTX, b *model.Booking) error {
	query := `INSERT INTO bookings (user_id, court_id, booking_code, date, total_amount, status)
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`
	return tx.QueryRow(ctx, query, b.UserID, b.CourtID, b.BookingCode, b.Date, b.TotalAmount, b.Status).
		Scan(&b.ID, &b.CreatedAt)
}

func (r *bookingPostgresRepo) FindByID(ctx context.Context, tx DBTX, id int64) (*model.Booking, error) {
	query := `SELECT id, user_id, court_id, booking_code, date::text, total_amount, status, created_at
	          FROM bookings WHERE id = $1`
	var b model.Booking
	err := tx.QueryRow(ctx, query, id).Scan(&b.ID, &b.UserID, &b.CourtID, &b.BookingCode, &b.Date, &b.TotalAmount, &b.Status, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

// Mengunci slot jam secara atomik. Jika salah satu slot sudah tidak tersedia, kembalikan ErrConflict.
func (r *bookingPostgresRepo) LockAndAssignSlots(ctx context.Context, tx DBTX, courtID int64, date string, slotIDs []int64, bookingID int64) error {
	query := `UPDATE court_slots 
	          SET is_available = false, booking_id = $1, version = version + 1
	          WHERE court_id = $2 AND date = $3 AND id = ANY($4) AND is_available = true`
	tag, err := tx.Exec(ctx, query, bookingID, courtID, date, slotIDs)
	if err != nil {
		return fmt.Errorf("lockSlots.Exec: %w", err)
	}
	if int(tag.RowsAffected()) != len(slotIDs) {
		return ErrConflict // Satu atau lebih slot telah dibooking pengguna lain!
	}
	return nil
}

// SwapSlots menukar slot lama ke slot baru bebas deadlock (kunci slot berurutan dari ID terkecil)
func (r *bookingPostgresRepo) SwapSlots(ctx context.Context, tx DBTX, bookingID, oldSlotID, newSlotID int64) error {
	firstID, secondID := oldSlotID, newSlotID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	// 1. Lock kedua slot secara eksplisit dalam urutan ID terurut
	lockQuery := `SELECT id FROM court_slots WHERE id IN ($1, $2) ORDER BY id ASC FOR UPDATE`
	rows, err := tx.Query(ctx, lockQuery, firstID, secondID)
	if err != nil {
		return err
	}
	rows.Close()

	// 2. Lepaskan slot lama
	releaseQuery := `UPDATE court_slots SET is_available = true, booking_id = NULL, version = version + 1 
	                 WHERE id = $1 AND booking_id = $2`
	tagRel, err := tx.Exec(ctx, releaseQuery, oldSlotID, bookingID)
	if err != nil || tagRel.RowsAffected() == 0 {
		return ErrNotFound
	}

	// 3. Ambil slot baru
	claimQuery := `UPDATE court_slots SET is_available = false, booking_id = $1, version = version + 1 
	               WHERE id = $2 AND is_available = true`
	tagClaim, err := tx.Exec(ctx, claimQuery, bookingID, newSlotID)
	if err != nil || tagClaim.RowsAffected() == 0 {
		return ErrConflict // Slot tujuan sudah tidak tersedia
	}

	return nil
}

func (r *bookingPostgresRepo) CountUserHoursByDate(ctx context.Context, tx DBTX, userID int64, date string) (int, error) {
	query := `SELECT COUNT(cs.id) 
	          FROM court_slots cs
	          JOIN bookings b ON b.id = cs.booking_id
	          WHERE b.user_id = $1 AND b.date = $2`
	var count int
	err := tx.QueryRow(ctx, query, userID, date).Scan(&count)
	return count, err
}

func (r *bookingPostgresRepo) HasUserBookedCourt(ctx context.Context, userID, courtID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM bookings WHERE user_id = $1 AND court_id = $2 AND status = 'CONFIRMED')`
	var exists bool
	err := r.pool.QueryRow(ctx, query, userID, courtID).Scan(&exists)
	return exists, err
}

// Keyset / Cursor Pagination berkinerja tinggi
func (r *bookingPostgresRepo) FindBookingsKeyset(ctx context.Context, userID int64, limit int, cursor *model.Cursor) ([]model.Booking, error) {
	var query string
	var args []any

	if cursor == nil {
		query = `SELECT id, user_id, court_id, booking_code, date::text, total_amount, status, created_at
		         FROM bookings
		         WHERE user_id = $1
		         ORDER BY created_at DESC, id DESC
		         LIMIT $2`
		args = []any{userID, limit}
	} else {
		query = `SELECT id, user_id, court_id, booking_code, date::text, total_amount, status, created_at
		         FROM bookings
		         WHERE user_id = $1 AND (created_at, id) < ($2, $3)
		         ORDER BY created_at DESC, id DESC
		         LIMIT $4`
		args = []any{userID, cursor.CreatedAt, cursor.ID, limit}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.CourtID, &b.BookingCode, &b.Date, &b.TotalAmount, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

// Sesi Mabar (Sparring)
func (r *bookingPostgresRepo) CreateSparring(ctx context.Context, tx DBTX, s *model.Sparring) error {
	query := `INSERT INTO sparrings (host_id, court_id, title, date, start_time, end_time, max_participants, current_participants, price_per_person, status)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, 'OPEN') RETURNING id, created_at`
	err := tx.QueryRow(ctx, query, s.HostID, s.CourtID, s.Title, s.Date, s.StartTime, s.EndTime, s.MaxParticipants, s.PricePerPerson).
		Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO sparring_members (sparring_id, user_id) VALUES ($1, $2)`, s.ID, s.HostID)
	return err
}

func (r *bookingPostgresRepo) FindAllOpenSparrings(ctx context.Context) ([]model.Sparring, error) {
	query := `SELECT id, host_id, court_id, title, date::text, start_time::text, end_time::text, max_participants, current_participants, price_per_person, status, created_at
	          FROM sparrings WHERE status = 'OPEN' AND current_participants < max_participants ORDER BY date ASC, start_time ASC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Sparring
	for rows.Next() {
		var s model.Sparring
		if err := rows.Scan(&s.ID, &s.HostID, &s.CourtID, &s.Title, &s.Date, &s.StartTime, &s.EndTime, &s.MaxParticipants, &s.CurrentParticipants, &s.PricePerPerson, &s.Status, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func (r *bookingPostgresRepo) FindSparringByID(ctx context.Context, id int64) (*model.Sparring, error) {
	query := `SELECT id, host_id, court_id, title, date::text, start_time::text, end_time::text, max_participants, current_participants, price_per_person, status, created_at
	          FROM sparrings WHERE id = $1`
	var s model.Sparring
	err := r.pool.QueryRow(ctx, query, id).
		Scan(&s.ID, &s.HostID, &s.CourtID, &s.Title, &s.Date, &s.StartTime, &s.EndTime, &s.MaxParticipants, &s.CurrentParticipants, &s.PricePerPerson, &s.Status, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *bookingPostgresRepo) JoinSparringAtomic(ctx context.Context, tx DBTX, sparringID, userID int64) error {
	query := `UPDATE sparrings 
	          SET current_participants = current_participants + 1,
	              status = CASE WHEN current_participants + 1 >= max_participants THEN 'FULL' ELSE status END
	          WHERE id = $1 AND current_participants < max_participants AND status = 'OPEN'`
	tag, err := tx.Exec(ctx, query, sparringID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict // Kuota sudah penuh
	}

	_, err = tx.Exec(ctx, `INSERT INTO sparring_members (sparring_id, user_id) VALUES ($1, $2)`, sparringID, userID)
	return err
}
```

```bash
git add app/repository/booking_repository.go
git commit -m "feat(booking): implement BookingRepository with atomic slot locking, deadlock-free swap, and sparring methods"
```

---

### Micro-Commit 4: Pure Business Rules & Table-Driven Unit Tests

Buat file **`app/service/booking_rules.go`**:

```go
package service

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrPastDate         = errors.New("tanggal pemesanan tidak boleh di masa lampau")
	ErrExceedDailyLimit = errors.New("total pemesanan melebihi batas kuota 4 jam per hari")
	ErrEmptySlots       = errors.New("minimal memilih 1 slot jam")
	ErrTooManySlots     = errors.New("maksimal memilih 4 slot jam per transaksi")
	ErrSameSlotSwap     = errors.New("slot baru tidak boleh sama dengan slot lama")
	ErrInvalidParticipants  = errors.New("jumlah peserta sparring minimal 2 orang")
	ErrEmptyTitle       = errors.New("judul sparring tidak boleh kosong")
)

// ValidateBookingRequest memvalidasi batas kuota dan tanggal pemesanan (Pure Logic)
func ValidateBookingRequest(bookingDateStr string, slotCount int, existingBookedHours int) error {
	if _, err := time.Parse("2006-01-02", bookingDateStr); err != nil {
		return errors.New("format tanggal tidak valid, gunakan YYYY-MM-DD")
	}

	todayStr := time.Now().Format("2006-01-02")
	if bookingDateStr < todayStr {
		return ErrPastDate
	}

	if slotCount < 1 {
		return ErrEmptySlots
	}
	if slotCount > 4 {
		return ErrTooManySlots
	}

	if existingBookedHours+slotCount > 4 {
		return ErrExceedDailyLimit
	}

	return nil
}

// ValidateSwapRequest memvalidasi integritas penukaran slot jam
func ValidateSwapRequest(bookingDateStr string, oldSlotID, newSlotID int64) error {
	if oldSlotID == newSlotID {
		return ErrSameSlotSwap
	}

	if _, err := time.Parse("2006-01-02", bookingDateStr); err != nil {
		return errors.New("format tanggal tidak valid, gunakan YYYY-MM-DD")
	}

	todayStr := time.Now().Format("2006-01-02")
	if bookingDateStr < todayStr {
		return ErrPastDate
	}

	return nil
}

// ValidateSparringCreation memvalidasi aturan pembuatan sesi mabar sparring
func ValidateSparringCreation(title string, maxParticipants int, dateStr string) error {
	if strings.TrimSpace(title) == "" {
		return ErrEmptyTitle
	}
	if maxParticipants < 2 {
		return ErrInvalidParticipants
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return errors.New("format tanggal tidak valid, gunakan YYYY-MM-DD")
	}
	todayStr := time.Now().Format("2006-01-02")
	if dateStr < todayStr {
		return ErrPastDate
	}
	return nil
}
```

Buat file **`app/service/booking_rules_test.go`**:

```go
package service

import (
	"errors"
	"testing"
	"time"
)

func TestValidateBookingRequest(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	tests := []struct {
		name         string
		date         string
		slots        int
		currentHours int
		wantErr      error
	}{
		{"Valid 2 Hours", tomorrow, 2, 0, nil},
		{"Valid Remaining Quota", tomorrow, 2, 2, nil},
		{"Past Date", yesterday, 1, 0, ErrPastDate},
		{"Zero Slots", tomorrow, 0, 0, ErrEmptySlots},
		{"More than 4 Slots", tomorrow, 5, 0, ErrTooManySlots},
		{"Exceed 4 Hours Daily Limit", tomorrow, 3, 2, ErrExceedDailyLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBookingRequest(tt.date, tt.slots, tt.currentHours)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateSwapRequest(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	tests := []struct {
		name      string
		date      string
		oldSlotID int64
		newSlotID int64
		wantErr   error
	}{
		{"Valid Swap", tomorrow, 1, 2, nil},
		{"Same Slot Swap", tomorrow, 1, 1, ErrSameSlotSwap},
		{"Past Date Swap", yesterday, 1, 2, ErrPastDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSwapRequest(tt.date, tt.oldSlotID, tt.newSlotID)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateSparringCreation(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	tests := []struct {
		name            string
		title           string
		maxParticipants int
		date            string
		wantErr         error
	}{
		{"Valid Sparring", "Mabar Futsal Santai", 10, tomorrow, nil},
		{"Empty Title", "   ", 10, tomorrow, ErrEmptyTitle},
		{"Too Few Participants", "Mabar Badminton", 1, tomorrow, ErrInvalidParticipants},
		{"Past Date Sparring", "Mabar Tennis", 4, yesterday, ErrPastDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSparringCreation(tt.title, tt.maxParticipants, tt.date)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}
```

```bash
git add app/service/booking_rules.go app/service/booking_rules_test.go
git commit -m "test(booking): implement quota, swap, sparring rules with table-driven unit tests"
```

---

### Micro-Commit 5: Fiber HTTP Handlers BookingService, ACID Transaction & Ekspor CSV

Buat file **`app/service/booking_service.go`**:

```go
package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)

type BookingService struct {
	bookingRepo repository.BookingRepository
	courtRepo   repository.CourtRepository
	txManager   *repository.TxManager
	logger      *slog.Logger
}

func NewBookingService(
	bookingRepo repository.BookingRepository,
	courtRepo repository.CourtRepository,
	txManager *repository.TxManager,
	logger *slog.Logger,
) *BookingService {
	return &BookingService{
		bookingRepo: bookingRepo,
		courtRepo:   courtRepo,
		txManager:   txManager,
		logger:      logger,
	}
}

// CreateBooking godoc
// @Summary Buat reservasi lapangan baru (ACID Transaction)
// @Security BearerAuth
// @Tags Bookings
// @Accept json
// @Produce json
// @Param request body model.CreateBookingRequest true "Data Pemesanan"
// @Success 201 {object} model.WebResponse
// @Router /bookings [post]
func (s *BookingService) CreateBooking(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.CreateBookingRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	court, err := s.courtRepo.FindByID(ctx, req.CourtID)
	if err != nil {
		return helper.NotFound("lapangan tidak ditemukan")
	}

	bookingCode := fmt.Sprintf("CM-%d-%d", time.Now().Unix(), user.ID)
	totalAmount := float64(len(req.SlotIDs)) * court.HourlyRate

	booking := model.Booking{
		UserID:      user.ID,
		CourtID:     req.CourtID,
		BookingCode: bookingCode,
		Date:        req.Date,
		TotalAmount: totalAmount,
		Status:      "CONFIRMED",
	}

	// ACID Transaction dengan auto-retry deadlock 40001/40P01
	err = s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		bookedHours, err := s.bookingRepo.CountUserHoursByDate(ctx, tx, user.ID, req.Date)
		if err != nil {
			return err
		}
		if err := ValidateBookingRequest(req.Date, len(req.SlotIDs), bookedHours); err != nil {
			return helper.Conflict(err.Error())
		}

		if err := s.bookingRepo.Create(ctx, tx, &booking); err != nil {
			return err
		}

		return s.bookingRepo.LockAndAssignSlots(ctx, tx, req.CourtID, req.Date, req.SlotIDs, booking.ID)
	})

	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("slot jam telah dipesan pengguna lain, silakan pilih slot lain")
	} else if err != nil {
		var appErr *helper.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return helper.Internal(err)
	}

	return helper.Created(c, "booking berhasil dibuat", booking, "")
}

// ListUserBookings godoc
// @Summary Riwayat booking pengguna (Keyset Cursor & CSV Content Negotiation)
// @Security BearerAuth
// @Tags Bookings
// @Param cursor query string false "Base64 Keyset Cursor"
// @Param limit query int false "Batas jumlah data (default 10)"
// @Success 200 {object} model.WebResponse
// @Router /bookings/my [get]
func (s *BookingService) ListUserBookings(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return err
	}

	format, err := helper.Negotiate(c, helper.FormatJSON, helper.FormatCSV)
	if err != nil {
		return err
	}

	bookings, err := s.bookingRepo.FindBookingsKeyset(ctx, user.ID, q.Limit+1, q.After)
	if err != nil {
		return helper.Internal(err)
	}

	// Content Negotiation: Ekspor CSV jika diminta client
	if format == helper.FormatCSV {
		return helper.WriteBookingsCSV(c, bookings)
	}

	hasMore := len(bookings) > q.Limit
	if hasMore {
		bookings = bookings[:q.Limit]
	}

	var nextCursor string
	if hasMore && len(bookings) > 0 {
		nextItem := bookings[len(bookings)-1]
		nextCursor = helper.EncodeCursor(nextItem.CreatedAt, nextItem.ID)
	}

	meta := &model.CursorMeta{
		Limit:      q.Limit,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}

	return helper.SuccessCursor(c, "riwayat booking berhasil diambil", bookings, meta)
}

// SwapBooking godoc
// @Summary Tukar slot jadwal pemesanan lapangan (Deadlock-Free Sorted Slot Locks)
// @Security BearerAuth
// @Tags Bookings
// @Accept json
// @Produce json
// @Param request body model.SwapBookingRequest true "Data Penukaran Slot"
// @Success 200 {object} model.WebResponse
// @Router /bookings/swap [post]
func (s *BookingService) SwapBooking(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.SwapBookingRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	if err := ValidateSwapRequest(req.Date, req.OldSlotID, req.NewSlotID); err != nil {
		return helper.BadRequest(err.Error())
	}

	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		// Validasi kepemilikan booking
		booking, err := s.bookingRepo.FindByID(ctx, tx, req.BookingID)
		if err != nil {
			return helper.NotFound("booking tidak ditemukan")
		}
		if booking.UserID != user.ID {
			return helper.Forbidden("tidak diizinkan mengubah booking pengguna lain")
		}
		if booking.Status != "CONFIRMED" {
			return helper.Conflict("hanya booking dengan status CONFIRMED yang dapat ditukar")
		}

		return s.bookingRepo.SwapSlots(ctx, tx, req.BookingID, req.OldSlotID, req.NewSlotID)
	})

	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("slot baru sudah terisi atau slot lama tidak valid")
	} else if errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("booking tidak ditemukan")
	} else if err != nil {
		var appErr *helper.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "slot booking berhasil ditukar", nil)
}

// CreateSparring godoc
// @Summary Buat sesi mabar / sparring baru dari booking yang terkonfirmasi
// @Security BearerAuth
// @Tags Sparrings
// @Accept json
// @Produce json
// @Param request body model.CreateSparringRequest true "Data Sesi Sparring"
// @Success 201 {object} model.WebResponse
// @Router /sparrings [post]
func (s *BookingService) CreateSparring(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.CreateSparringRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	if _, err := s.courtRepo.FindByID(ctx, req.CourtID); err != nil {
		return helper.NotFound("lapangan tidak ditemukan")
	}

	if err := ValidateSparringCreation(req.Title, req.MaxParticipants, req.Date); err != nil {
		return helper.BadRequest(err.Error())
	}

	sparring := model.Sparring{
		HostID:              user.ID,
		CourtID:             req.CourtID,
		Title:               req.Title,
		Date:                req.Date,
		StartTime:           req.StartTime,
		EndTime:             req.EndTime,
		MaxParticipants:     req.MaxParticipants,
		CurrentParticipants: 1,
		PricePerPerson:      req.PricePerPerson,
		Status:              "OPEN",
	}

	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		return s.bookingRepo.CreateSparring(ctx, tx, &sparring)
	})

	if err != nil {
		var appErr *helper.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return helper.Internal(err)
	}

	return helper.Created(c, "sesi sparring berhasil dibuka", sparring, "")
}

// ListSparrings godoc
// @Summary Ambil daftar sesi mabar sparring yang masih berstatus OPEN
// @Tags Sparrings
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /sparrings [get]
func (s *BookingService) ListSparrings(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	list, err := s.bookingRepo.FindAllOpenSparrings(ctx)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "daftar sesi sparring terbuka berhasil diambil", list)
}

// JoinSparring godoc
// @Summary Bergabung ke sesi mabar sparring
// @Security BearerAuth
// @Tags Sparrings
// @Param id path int true "Sparring ID"
// @Success 200 {object} model.WebResponse
// @Router /sparrings/{id}/join [post]
func (s *BookingService) JoinSparring(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	sparringID, ok := helper.ParamID(c, "id")
	if !ok {
		return helper.BadRequest("ID sparring tidak valid")
	}

	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		return s.bookingRepo.JoinSparringAtomic(ctx, tx, sparringID, user.ID)
	})

	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("kuota peserta sparring sudah penuh atau sesi sudah ditutup")
	} else if err != nil {
		var appErr *helper.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "berhasil bergabung ke sesi sparring", nil)
}
```

```bash
git add app/service/booking_service.go
git commit -m "feat(booking): implement BookingService with ACID TxManager, swap slots, sparring endpoints, and CSV RFC 4180 export"
```

**Push Branch PR 1 & Buka PR #3:**
```bash
git push -u origin feat/booking-reservation
```

#### Cara A: Menggunakan GitHub CLI (`gh`) di Terminal
```bash
gh pr create --base develop --head feat/booking-reservation \
  --title "feat(booking): implement acid booking reservation, keyset pagination, and csv export" \
  --body ""
```

#### Cara B: Melalui GitHub Web UI
1. Klik link pembuatan PR yang muncul di log terminal setelah `git push`, atau buka repositori di web GitHub lalu klik **Compare & pull request**.
2. Pastikan target branch: **base: `develop`** $\leftarrow$ **compare: `feat/booking-reservation`**.
3. Isi **Title**:
   ```text
   feat(booking): implement acid booking reservation, keyset pagination, and csv export
   ```
4. Kolom **Description** dapat dikosongkan, lalu klik **Create pull request**.

*(Setelah PR #3 disetujui & di-merge oleh Git Master ke `develop`, lanjutkan ke PR 2 di bawah).*

---

## 4. Eksekusi PR 2: `feat/outbox-worker` (Transactional Outbox & Asynq Worker)

### Persiapan Branch PR 2:
```bash
git checkout develop
git pull origin develop
git checkout -b feat/outbox-worker
```

---

### Micro-Commit 6: DDL Migrasi Transactional Outbox & Notifications

Buat file **`migrations/006_outbox_and_notifications.sql`**:

```sql
-- outbox: event dicatat di dalam transaction ACID yang sama dengan booking
CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ NULL
);

-- Partial index agar scanning event yang belum dikirim sangat cepat
CREATE INDEX IF NOT EXISTS outbox_unpublished_idx
    ON outbox (id) WHERE published_at IS NULL;

-- notifications: bukti idempotensi bahwa event SUDAH dikirimkan
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    outbox_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL,
    subject VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notifications_outbox_unique UNIQUE (outbox_id)
);
```

```bash
git add migrations/006_outbox_and_notifications.sql
git commit -m "feat(outbox): create outbox with partial index and notifications table with unique outbox_id"
```

---

### Micro-Commit 7: Model Event Outbox & Notifikasi

Buat file **`app/model/notification.go`**:

```go
package model

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	ID          int64           `json:"id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at"`
}

type NotificationJob struct {
	OutboxID  int64           `json:"outbox_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type Notification struct {
	ID        int64     `json:"id"`
	OutboxID  int64     `json:"outbox_id"`
	UserID    int64     `json:"user_id"`
	Channel   string    `json:"channel"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	SentAt    time.Time `json:"sent_at"`
}

type BookingCreatedPayload struct {
	BookingID   int64   `json:"booking_id"`
	BookingCode string  `json:"booking_code"`
	UserID      int64   `json:"user_id"`
	CourtID     int64   `json:"court_id"`
	Date        string  `json:"date"`
	TotalAmount float64 `json:"total_amount"`
}
```

```bash
git add app/model/notification.go
git commit -m "feat(notification): define OutboxEvent, Notification, and event payload DTOs"
```

---

### Micro-Commit 8: Repository Outbox & Notification (PostgreSQL)

Buat file **`app/repository/outbox_repository.go`**:
```go
package repository

import (
	"context"
	"time"

	"courtmaster-backend/app/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository interface {
	CreateTx(ctx context.Context, tx DBTX, event *model.OutboxEvent) error
	FetchUnpublished(ctx context.Context, limit int) ([]model.OutboxEvent, error)
	MarkPublished(ctx context.Context, id int64) error
	CountPending(ctx context.Context) (int64, error)
}

type outboxPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) OutboxRepository {
	return &outboxPostgresRepo{pool: pool}
}

func (r *outboxPostgresRepo) CreateTx(ctx context.Context, tx DBTX, event *model.OutboxEvent) error {
	query := `INSERT INTO outbox (event_type, payload) VALUES ($1, $2) RETURNING id, created_at`
	return tx.QueryRow(ctx, query, event.EventType, event.Payload).Scan(&event.ID, &event.CreatedAt)
}

func (r *outboxPostgresRepo) FetchUnpublished(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	query := `SELECT id, event_type, payload, created_at, published_at
	          FROM outbox
	          WHERE published_at IS NULL
	          ORDER BY id ASC
	          LIMIT $1
	          FOR UPDATE SKIP LOCKED`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.OutboxEvent
	for rows.Next() {
		var e model.OutboxEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.CreatedAt, &e.PublishedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *outboxPostgresRepo) MarkPublished(ctx context.Context, id int64) error {
	query := `UPDATE outbox SET published_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *outboxPostgresRepo) CountPending(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM outbox WHERE published_at IS NULL`
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	return count, err
}
```

Buat file **`app/repository/notification_repository.go`**:
```go
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

// Claim menerapkan pola idempotensi atomik Modul 11 (ON CONFLICT DO NOTHING)
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
		return false, fmt.Errorf("mencatat notifikasi: %w", err)
	}
	n.ID = id
	return true, nil
}

func (r *notificationPostgresRepo) Create(ctx context.Context, tx DBTX, n *model.Notification) error {
	query := `INSERT INTO notifications (outbox_id, user_id, channel, subject, body)
	          VALUES ($1, $2, $3, $4, $5) RETURNING id, sent_at`
	return tx.QueryRow(ctx, query, n.OutboxID, n.UserID, n.Channel, n.Subject, n.Body).
		Scan(&n.ID, &n.SentAt)
}

func (r *notificationPostgresRepo) FindByUserID(ctx context.Context, userID int64) ([]model.Notification, error) {
	query := `SELECT id, outbox_id, user_id, channel, subject, body, sent_at
	          FROM notifications
	          WHERE user_id = $1
	          ORDER BY sent_at DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.OutboxID, &n.UserID, &n.Channel, &n.Subject, &n.Body, &n.SentAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, nil
}
```

```bash
git add app/repository/outbox_repository.go app/repository/notification_repository.go
git commit -m "feat(outbox): implement OutboxRepository with SKIP LOCKED scan and NotificationRepository"
```

---

### Micro-Commit 9: Asynq Relay Producer & Consumer Worker Handler

Buat file **`app/service/outbox_relay.go`**:
```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/config"
	"github.com/hibiken/asynq"
)

type OutboxRelay struct {
	repo   repository.OutboxRepository
	client *asynq.Client
	logger *slog.Logger
}

func NewOutboxRelay(repo repository.OutboxRepository, client *asynq.Client, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{repo: repo, client: client, logger: logger}
}

// Run menjalankan loop background relay berkala secara otomatis (Modul 11)
func (r *OutboxRelay) Run(ctx context.Context) {
	interval := time.Duration(config.GetEnvInt("RELAY_INTERVAL_MS", 3000)) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if count, err := r.PublishPending(ctx); err != nil {
				r.logger.Error("outbox_relay_error", slog.String("error", err.Error()))
			} else if count > 0 {
				r.logger.Info("outbox_auto_relayed", slog.Int("count", count))
			}
		}
	}
}

func (r *OutboxRelay) PublishPending(ctx context.Context) (int, error) {
	batchSize := config.GetEnvInt("RELAY_BATCH", 50)
	events, err := r.repo.FetchUnpublished(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	publishedCount := 0
	for _, event := range events {
		// Bungkus dalam model.NotificationJob DTO (Modul 11)
		job := model.NotificationJob{
			OutboxID:  event.ID,
			EventType: event.EventType,
			Payload:   event.Payload,
		}
		jobBytes, err := json.Marshal(job)
		if err != nil {
			r.logger.Error("failed_to_serialize_job", slog.String("error", err.Error()))
			continue
		}

		task := asynq.NewTask("booking:send_notification", jobBytes,
			asynq.TaskID(fmt.Sprintf("outbox-%d", event.ID)),
			asynq.Retention(24*time.Hour),
			asynq.MaxRetry(3),
		)

		if _, err := r.client.EnqueueContext(ctx, task); err != nil {
			r.logger.Error("failed_to_enqueue_outbox_task", slog.String("error", err.Error()))
			continue
		}

		_ = r.repo.MarkPublished(ctx, event.ID)
		publishedCount++
	}
	return publishedCount, nil
}
```

Buat file **`app/service/notification_worker.go`**:
```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationWorker struct {
	pool      *pgxpool.Pool
	txManager *repository.TxManager
	notifRepo repository.NotificationRepository
	logger    *slog.Logger
}

func NewNotificationWorker(pool *pgxpool.Pool, txManager *repository.TxManager, logger *slog.Logger) *NotificationWorker {
	return &NotificationWorker{
		pool:      pool,
		txManager: txManager,
		notifRepo: repository.NewNotificationRepository(pool),
		logger:    logger,
	}
}

func (w *NotificationWorker) HandleSendNotification(ctx context.Context, t *asynq.Task) error {
	var job model.NotificationJob
	if err := json.Unmarshal(t.Payload(), &job); err != nil {
		// Data rusak tidak perlu di-retry lagi (Modul 11)
		return fmt.Errorf("%w: payload task rusak: %v", asynq.SkipRetry, err)
	}

	var payload model.BookingCreatedPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("%w: payload booking rusak: %v", asynq.SkipRetry, err)
	}

	notif := model.Notification{
		OutboxID: job.OutboxID,
		UserID:   payload.UserID,
		Channel:  "EMAIL",
		Subject:  "Konfirmasi Pemesanan Lapangan " + payload.BookingCode,
		Body:     fmt.Sprintf("Booking Anda sebesar Rp %.2f untuk tanggal %s berhasil dikonfirmasi.", payload.TotalAmount, payload.Date),
	}

	// Idempotent Claim pattern Modul 11 (ON CONFLICT DO NOTHING)
	var claimed bool
	err := w.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		var err error
		claimed, err = w.notifRepo.Claim(ctx, tx, &notif)
		return err
	})
	if err != nil {
		return err
	}
	if !claimed {
		w.logger.Info("idempotent_duplicate_notification_skipped", slog.Int64("outbox_id", job.OutboxID))
		return nil
	}

	w.logger.Info("notification_sent_successfully", slog.String("booking_code", payload.BookingCode))
	return nil
}
```

Sekarang integrasikan `OutboxRepository` ke dalam file **`app/service/booking_service.go`** (agar setiap pembuatan booking baru otomatis mencatat event outbox secara transaksional sesuai Modul 11):

1. Tambahkan `"encoding/json"` pada blok `import` di bagian atas berkas `app/service/booking_service.go`:
```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)
```

2. Perbarui definisi struct dan konstruktor di `app/service/booking_service.go`:
```go
type BookingService struct {
	bookingRepo repository.BookingRepository
	courtRepo   repository.CourtRepository
	outboxRepo  repository.OutboxRepository
	txManager   *repository.TxManager
	logger      *slog.Logger
}

func NewBookingService(
	bookingRepo repository.BookingRepository,
	courtRepo repository.CourtRepository,
	outboxRepo repository.OutboxRepository,
	txManager *repository.TxManager,
	logger *slog.Logger,
) *BookingService {
	return &BookingService{
		bookingRepo: bookingRepo,
		courtRepo:   courtRepo,
		outboxRepo:  outboxRepo,
		txManager:   txManager,
		logger:      logger,
	}
}
```
3. Tambahkan pencatatan outbox di dalam callback `txManager.WithTx` pada method `CreateBooking`:
```go
		if err := s.bookingRepo.LockAndAssignSlots(ctx, tx, req.CourtID, req.Date, req.SlotIDs, booking.ID); err != nil {
			return err
		}

		// Catat event outbox via OutboxRepository di transaksi yang sama (SOP Rule 5: No raw SQL in service)
		if s.outboxRepo != nil {
			payload, _ := json.Marshal(model.BookingCreatedPayload{
				BookingID:   booking.ID,
				BookingCode: booking.BookingCode,
				UserID:      user.ID,
				CourtID:     req.CourtID,
				Date:        req.Date,
				TotalAmount: totalAmount,
			})
			outboxEvent := model.OutboxEvent{
				EventType: "booking:created",
				Payload:   payload,
			}
			return s.outboxRepo.CreateTx(ctx, tx, &outboxEvent)
		}
		return nil
```

```bash
git add app/service/booking_service.go app/service/outbox_relay.go app/service/notification_worker.go
git commit -m "feat(worker): implement Asynq relay producer, consumer worker, and integrate outbox to booking service"
```

---

### Micro-Commit 10: Notification HTTP Service & Dual-Process CLI Runner (`worker.go`)

Buat file **`app/service/notification_service.go`**:
```go
package service

import (
	"log/slog"

	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

type NotificationService struct {
	notifRepo  repository.NotificationRepository
	outboxRepo repository.OutboxRepository
	relay      *OutboxRelay
	inspector  *asynq.Inspector
	logger     *slog.Logger
}

func NewNotificationService(
	notifRepo repository.NotificationRepository,
	outboxRepo repository.OutboxRepository,
	relay *OutboxRelay,
	inspector *asynq.Inspector,
	logger *slog.Logger,
) *NotificationService {
	return &NotificationService{
		notifRepo:  notifRepo,
		outboxRepo: outboxRepo,
		relay:      relay,
		inspector:  inspector,
		logger:     logger,
	}
}

// ListNotifications godoc
// @Summary Ambil daftar notifikasi milik pengguna yang sedang login
// @Security BearerAuth
// @Tags Notifications
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /notifications [get]
func (s *NotificationService) ListNotifications(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	list, err := s.notifRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar notifikasi berhasil diambil", list)
}

// RetryOutbox godoc
// @Summary Trigger manual relay event outbox yang tertunda (Admin Only)
// @Security BearerAuth
// @Tags Notifications
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /notifications/outbox/retry [post]
func (s *NotificationService) RetryOutbox(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	count, err := s.relay.PublishPending(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "relay outbox berhasil dijalankan", map[string]int{"published": count})
}

// GetQueueStats godoc
// @Summary Monitoring statistik antrean Asynq dan outbox event (Admin Only)
// @Security BearerAuth
// @Tags Admin
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /admin/queue [get]
func (s *NotificationService) GetQueueStats(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	pendingOutbox, err := s.outboxRepo.CountPending(ctx)
	if err != nil {
		return helper.Internal(err)
	}

	queueInfo, err := s.inspector.GetQueueInfo("default")
	if err != nil {
		s.logger.Warn("gagal_mengambil_info_antrean_redis", slog.String("error", err.Error()))
		return helper.Success(c, fiber.StatusOK, "statistik antrean parsial", map[string]any{
			"pending_outbox": pendingOutbox,
			"asynq_queue":    nil,
		})
	}

	return helper.Success(c, fiber.StatusOK, "statistik antrean berhasil diambil", map[string]any{
		"pending_outbox": pendingOutbox,
		"asynq_queue": map[string]any{
			"active":    queueInfo.Active,
			"pending":   queueInfo.Pending,
			"scheduled": queueInfo.Scheduled,
			"retry":     queueInfo.Retry,
			"archived":  queueInfo.Archived,
			"completed": queueInfo.Completed,
			"size":      queueInfo.Size,
		},
	})
}
```

Buat file **`worker.go`** di root proyek:
```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"courtmaster-backend/app/repository"
	"courtmaster-backend/app/service"
	"courtmaster-backend/config"
	"courtmaster-backend/database"
	"github.com/hibiken/asynq"
)

func startWorker(logger *slog.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx)
	if err != nil {
		logger.Error("worker_db_pool_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.GetEnv("REDIS_ADDR", "localhost:6379"),
		Password: config.GetEnv("REDIS_PASSWORD", ""),
		DB:       config.GetEnvInt("REDIS_DB", 0),
	}
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: config.GetEnvInt("WORKER_CONCURRENCY", 5),
		},
	)

	txManager := repository.NewTxManager(pool)
	workerHandler := service.NewNotificationWorker(pool, txManager, logger)

	mux := asynq.NewServeMux()
	mux.HandleFunc("booking:send_notification", workerHandler.HandleSendNotification)

	// Inisialisasi Outbox Relay dan jalankan background relay runner (Modul 11)
	outboxRepo := repository.NewOutboxRepository(pool)
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()
	relay := service.NewOutboxRelay(outboxRepo, asynqClient, logger)

	go relay.Run(ctx)

	logger.Info("asynq_worker_started", slog.String("redis", redisOpt.Addr))
	if err := srv.Run(mux); err != nil {
		logger.Error("asynq_worker_stopped", slog.String("error", err.Error()))
	}
}
```

```bash
git add app/service/notification_service.go worker.go
git commit -m "feat(cli): implement NotificationService, queue monitoring, and dual-process Asynq worker runner (-role=worker)"
```

**Push Branch PR 2 & Buka PR #5:**
```bash
git push -u origin feat/outbox-worker
```

#### Cara A: Menggunakan GitHub CLI (`gh`) di Terminal
```bash
gh pr create --base develop --head feat/outbox-worker \
  --title "feat(worker): implement transactional outbox pattern and asynq async worker" \
  --body ""
```

#### Cara B: Melalui GitHub Web UI
1. Klik link pembuatan PR yang muncul di log terminal setelah `git push`, atau buka repositori di web GitHub lalu klik **Compare & pull request**.
2. Pastikan target branch: **base: `develop`** $\leftarrow$ **compare: `feat/outbox-worker`**.
3. Isi **Title**:
   ```text
   feat(worker): implement transactional outbox pattern and asynq async worker
   ```
4. Kolom **Description** dapat dikosongkan, lalu klik **Create pull request**.

---

## 5. Ringkasan 10 Micro-Commits Anggota 3

| No | Commit Hash / Pesan | PR Target | Files yang Di-stage |
| :---: | :--- | :--- | :--- |
| **1** | `feat(booking): create bookings...` | PR #3 (`feat/booking-reservation`) | `migrations/005_bookings_and_sparrings.sql` |
| **2** | `feat(booking): define Booking, Sparring...` | PR #3 (`feat/booking-reservation`) | `app/model/booking.go` |
| **3** | `feat(booking): BookingRepository slot lock` | PR #3 (`feat/booking-reservation`) | `app/repository/booking_repository.go` |
| **4** | `test(booking): quota rules & unit tests` | PR #3 (`feat/booking-reservation`) | `app/service/booking_rules.go`, `booking_rules_test.go` |
| **5** | `feat(booking): BookingService ACID & CSV` | PR #3 (`feat/booking-reservation`) | `app/service/booking_service.go` |
| **6** | `feat(outbox): create outbox & notif DDL` | PR #5 (`feat/outbox-worker`) | `migrations/006_outbox_and_notifications.sql` |
| **7** | `feat(notification): define OutboxEvent...` | PR #5 (`feat/outbox-worker`) | `app/model/notification.go` |
| **8** | `feat(outbox): OutboxRepository SKIP LOCKED` | PR #5 (`feat/outbox-worker`) | `app/repository/outbox_repository.go`, `notification_repository.go` |
| **9** | `feat(worker): Asynq relay, worker & outbox integration`| PR #5 (`feat/outbox-worker`) | `app/service/booking_service.go`, `outbox_relay.go`, `notification_worker.go` |
| **10**| `feat(cli): NotificationService & worker.go` | PR #5 (`feat/outbox-worker`) | `app/service/notification_service.go`, `worker.go` |
