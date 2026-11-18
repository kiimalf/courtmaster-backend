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
	SwapSlots(ctx context.Context, tx DBTX, bookingID, courtID int64, date string, oldSlotID, newSlotID int64) error
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
	err := tx.QueryRow(ctx, query, b.UserID, b.CourtID, b.BookingCode, b.Date, b.TotalAmount, b.Status).
		Scan(&b.ID, &b.CreatedAt)
	if pgErrorCode(err) == codeUniqueViolation {
		return ErrConflict // booking_code bentrok, ulangi dengan kode baru
	}
	if err != nil {
		return fmt.Errorf("bookingRepo.Create: %w", err)
	}
	return nil
}

func (r *bookingPostgresRepo) FindByID(ctx context.Context, tx DBTX, id int64) (*model.Booking, error) {
	query := `SELECT id, user_id, court_id, booking_code, date::text, total_amount, status, created_at
	          FROM bookings WHERE id = $1`

	var b model.Booking
	err := tx.QueryRow(ctx, query, id).
		Scan(&b.ID, &b.UserID, &b.CourtID, &b.BookingCode, &b.Date, &b.TotalAmount, &b.Status, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("bookingRepo.FindByID: %w", err)
	}
	return &b, nil
}

// LockAndAssignSlots mengunci slot jam secara atomik. Jika salah satu slot sudah
// tidak tersedia (atau bukan milik court/tanggal yang diminta), seluruh transaksi
// dibatalkan melalui ErrConflict.
func (r *bookingPostgresRepo) LockAndAssignSlots(ctx context.Context, tx DBTX, courtID int64, date string, slotIDs []int64, bookingID int64) error {
	if len(slotIDs) == 0 {
		return ErrConflict
	}

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

// SwapSlots menukar slot lama ke slot baru secara bebas deadlock: kedua slot
// dikunci eksplisit dalam urutan ID menaik sebelum ditukar.
func (r *bookingPostgresRepo) SwapSlots(ctx context.Context, tx DBTX, bookingID, courtID int64, date string, oldSlotID, newSlotID int64) error {
	firstID, secondID := oldSlotID, newSlotID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	// 1. Kunci kedua slot dalam urutan ID terurut dan pastikan keduanya milik court/tanggal yang sama.
	lockQuery := `SELECT id FROM court_slots
	              WHERE id IN ($1, $2) AND court_id = $3 AND date = $4
	              ORDER BY id ASC FOR UPDATE`
	rows, err := tx.Query(ctx, lockQuery, firstID, secondID, courtID, date)
	if err != nil {
		return fmt.Errorf("swapSlots.lock: %w", err)
	}
	locked := 0
	for rows.Next() {
		locked++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("swapSlots.lockRows: %w", err)
	}
	if locked != 2 {
		return ErrNotFound // slot tidak ditemukan pada court/tanggal tersebut
	}

	// 2. Lepaskan slot lama (hanya jika memang dipegang booking ini).
	releaseQuery := `UPDATE court_slots SET is_available = true, booking_id = NULL, version = version + 1
	                 WHERE id = $1 AND booking_id = $2`
	tagRel, err := tx.Exec(ctx, releaseQuery, oldSlotID, bookingID)
	if err != nil {
		return fmt.Errorf("swapSlots.release: %w", err)
	}
	if tagRel.RowsAffected() == 0 {
		return ErrNotFound
	}

	// 3. Ambil slot baru.
	claimQuery := `UPDATE court_slots SET is_available = false, booking_id = $1, version = version + 1
	               WHERE id = $2 AND is_available = true`
	tagClaim, err := tx.Exec(ctx, claimQuery, bookingID, newSlotID)
	if err != nil {
		return fmt.Errorf("swapSlots.claim: %w", err)
	}
	if tagClaim.RowsAffected() == 0 {
		return ErrConflict // Slot tujuan sudah tidak tersedia
	}

	return nil
}

// CountUserHoursByDate menghitung jam terpakai user pada tanggal tertentu.
// Satu baris court_slots merepresentasikan satu jam operasional.
func (r *bookingPostgresRepo) CountUserHoursByDate(ctx context.Context, tx DBTX, userID int64, date string) (int, error) {
	query := `SELECT COUNT(cs.id)
	          FROM court_slots cs
	          JOIN bookings b ON b.id = cs.booking_id
	          WHERE b.user_id = $1 AND b.date = $2 AND b.status = 'CONFIRMED'`
	var count int
	if err := tx.QueryRow(ctx, query, userID, date).Scan(&count); err != nil {
		return 0, fmt.Errorf("bookingRepo.CountUserHoursByDate: %w", err)
	}
	return count, nil
}

func (r *bookingPostgresRepo) HasUserBookedCourt(ctx context.Context, userID, courtID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM bookings WHERE user_id = $1 AND court_id = $2 AND status = 'CONFIRMED')`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, userID, courtID).Scan(&exists); err != nil {
		return false, fmt.Errorf("bookingRepo.HasUserBookedCourt: %w", err)
	}
	return exists, nil
}

// FindBookingsKeyset melakukan Keyset / Cursor Pagination berkinerja tinggi
// dengan urutan (created_at DESC, id DESC).
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
		return nil, fmt.Errorf("bookingRepo.FindBookingsKeyset: %w", err)
	}
	defer rows.Close()

	list := make([]model.Booking, 0, limit)
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.CourtID, &b.BookingCode, &b.Date, &b.TotalAmount, &b.Status, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("bookingRepo.FindBookingsKeyset.scan: %w", err)
		}
		list = append(list, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bookingRepo.FindBookingsKeyset.rows: %w", err)
	}
	return list, nil
}

// CreateSparring membuat sesi mabar dan langsung mendaftarkan host sebagai peserta pertama.
func (r *bookingPostgresRepo) CreateSparring(ctx context.Context, tx DBTX, s *model.Sparring) error {
	query := `INSERT INTO sparrings (host_id, court_id, title, date, start_time, end_time, max_participants, current_participants, price_per_person, status)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, 'OPEN') RETURNING id, created_at`
	err := tx.QueryRow(ctx, query, s.HostID, s.CourtID, s.Title, s.Date, s.StartTime, s.EndTime, s.MaxParticipants, s.PricePerPerson).
		Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("bookingRepo.CreateSparring: %w", err)
	}

	_, err = tx.Exec(ctx, `INSERT INTO sparring_members (sparring_id, user_id) VALUES ($1, $2)`, s.ID, s.HostID)
	if err != nil {
		return fmt.Errorf("bookingRepo.CreateSparring.hostMember: %w", err)
	}
	return nil
}

func (r *bookingPostgresRepo) FindAllOpenSparrings(ctx context.Context) ([]model.Sparring, error) {
	query := `SELECT id, host_id, court_id, title, date::text, start_time::text, end_time::text, max_participants, current_participants, price_per_person, status, created_at
	          FROM sparrings WHERE status = 'OPEN' AND current_participants < max_participants ORDER BY date ASC, start_time ASC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("bookingRepo.FindAllOpenSparrings: %w", err)
	}
	defer rows.Close()

	list := make([]model.Sparring, 0)
	for rows.Next() {
		var s model.Sparring
		if err := rows.Scan(&s.ID, &s.HostID, &s.CourtID, &s.Title, &s.Date, &s.StartTime, &s.EndTime, &s.MaxParticipants, &s.CurrentParticipants, &s.PricePerPerson, &s.Status, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("bookingRepo.FindAllOpenSparrings.scan: %w", err)
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bookingRepo.FindAllOpenSparrings.rows: %w", err)
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
		return nil, fmt.Errorf("bookingRepo.FindSparringByID: %w", err)
	}
	return &s, nil
}

// JoinSparringAtomic mencegah double-join dan over-quota. Pendaftaran anggota
// dilakukan lebih dulu (ON CONFLICT DO NOTHING); bila user sudah terdaftar,
// transaksi dibatalkan sebelum counter kuota dinaikkan.
func (r *bookingPostgresRepo) JoinSparringAtomic(ctx context.Context, tx DBTX, sparringID, userID int64) error {
	memberQuery := `INSERT INTO sparring_members (sparring_id, user_id) VALUES ($1, $2)
	                ON CONFLICT (sparring_id, user_id) DO NOTHING`
	tagMember, err := tx.Exec(ctx, memberQuery, sparringID, userID)
	if err != nil {
		return fmt.Errorf("bookingRepo.JoinSparringAtomic.member: %w", err)
	}
	if tagMember.RowsAffected() == 0 {
		return ErrConflict // user sudah terdaftar pada sesi ini
	}

	quotaQuery := `UPDATE sparrings
	               SET current_participants = current_participants + 1,
	                   status = CASE WHEN current_participants + 1 >= max_participants THEN 'FULL' ELSE status END
	               WHERE id = $1 AND current_participants < max_participants AND status = 'OPEN'`
	tagQuota, err := tx.Exec(ctx, quotaQuery, sparringID)
	if err != nil {
		return fmt.Errorf("bookingRepo.JoinSparringAtomic.quota: %w", err)
	}
	if tagQuota.RowsAffected() == 0 {
		return ErrConflict // kuota sudah penuh atau sesi sudah ditutup
	}
	return nil
}
