package service

import (
	"crypto/rand"
	"encoding/hex"
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

// newBookingCode menghasilkan kode booking unik: CM-<unix>-<12 hex acak>.
// Panjang maksimal 26 karakter sehingga aman terhadap kolom VARCHAR(30).
func newBookingCode(userID int64) (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("membuat kode booking: %w", err)
	}
	return fmt.Sprintf("CM-%d-%s", time.Now().Unix(), hex.EncodeToString(buf)), nil
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

	// Validasi statis (format tanggal, jumlah slot, duplikasi) sebelum menyentuh database.
	if err := ValidateBookingRequest(req.Date, req.SlotIDs, 0); err != nil {
		return helper.BadRequest(err.Error())
	}

	court, err := s.courtRepo.FindByID(ctx, req.CourtID)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("lapangan tidak ditemukan")
	} else if err != nil {
		return helper.Internal(err)
	}
	if !court.IsActive {
		return helper.Conflict("lapangan sedang tidak aktif")
	}

	bookingCode, err := newBookingCode(user.ID)
	if err != nil {
		return helper.Internal(err)
	}

	booking := model.Booking{
		UserID:      user.ID,
		CourtID:     req.CourtID,
		BookingCode: bookingCode,
		Date:        req.Date,
		TotalAmount: float64(len(req.SlotIDs)) * court.HourlyRate,
		Status:      "CONFIRMED",
	}

	// ACID Transaction dengan auto-retry deadlock 40001/40P01 dari TxManager.
	err = s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		bookedHours, err := s.bookingRepo.CountUserHoursByDate(ctx, tx, user.ID, req.Date)
		if err != nil {
			return err
		}

		if err := ValidateBookingRequest(req.Date, req.SlotIDs, bookedHours); err != nil {
			if errors.Is(err, ErrExceedDailyLimit) {
				return helper.Conflict(err.Error())
			}
			return helper.BadRequest(err.Error())
		}

		if err := s.bookingRepo.Create(ctx, tx, &booking); err != nil {
			return err
		}

		if err := s.bookingRepo.LockAndAssignSlots(ctx, tx, req.CourtID, req.Date, req.SlotIDs, booking.ID); err != nil {
			return err
		}

		// Transactional Outbox: event dicatat pada transaksi ACID yang sama dengan
		// booking sehingga tidak ada dual-write dan tidak ada pesan yang hilang.
		if s.outboxRepo == nil {
			s.logger.Warn("outbox_repository_nil_booking_event_skipped",
				slog.Int64("booking_id", booking.ID),
			)
			return nil
		}

		payload, err := json.Marshal(model.BookingCreatedPayload{
			BookingID:   booking.ID,
			BookingCode: booking.BookingCode,
			UserID:      user.ID,
			CourtID:     req.CourtID,
			Date:        req.Date,
			TotalAmount: booking.TotalAmount,
		})
		if err != nil {
			return fmt.Errorf("serialisasi payload booking: %w", err)
		}

		return s.outboxRepo.CreateTx(ctx, tx, &model.OutboxEvent{
			EventType: model.EventTypeBookingCreated,
			Payload:   payload,
		})
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
// @Produce json
// @Produce text/csv
// @Param cursor query string false "Base64 Keyset Cursor"
// @Param limit query int false "Batas jumlah data (default 10, maksimal 100)"
// @Success 200 {object} model.WebResponse
// @Router /bookings [get]
func (s *BookingService) ListUserBookings(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return helper.BadRequest("parameter cursor/limit tidak valid")
	}

	format, err := helper.Negotiate(c, helper.FormatJSON, helper.FormatCSV)
	if err != nil {
		return helper.NotAcceptable(err.Error())
	}

	// Ambil limit+1 baris untuk mendeteksi has_more tanpa COUNT(*).
	bookings, err := s.bookingRepo.FindBookingsKeyset(ctx, user.ID, q.Limit+1, q.After)
	if err != nil {
		return helper.Internal(err)
	}

	hasMore := len(bookings) > q.Limit
	if hasMore {
		bookings = bookings[:q.Limit]
	}

	// Content Negotiation: unduh CSV RFC 4180 jika client meminta text/csv.
	if format == helper.FormatCSV {
		return helper.WriteBookingsCSV(c, bookings)
	}

	var nextCursor string
	if hasMore && len(bookings) > 0 {
		last := bookings[len(bookings)-1]
		nextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
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
		booking, err := s.bookingRepo.FindByID(ctx, tx, req.BookingID)
		if err != nil {
			return err
		}
		if booking.UserID != user.ID {
			return helper.Forbidden("tidak diizinkan mengubah booking pengguna lain")
		}
		if booking.Status != "CONFIRMED" {
			return helper.Conflict("hanya booking dengan status CONFIRMED yang dapat ditukar")
		}
		if booking.CourtID != req.CourtID || booking.Date != req.Date {
			return helper.BadRequest("court_id atau tanggal tidak sesuai dengan data booking")
		}

		return s.bookingRepo.SwapSlots(ctx, tx, req.BookingID, req.CourtID, req.Date, req.OldSlotID, req.NewSlotID)
	})

	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("slot baru sudah terisi, silakan pilih slot lain")
	} else if errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("booking atau slot tidak ditemukan")
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
// @Summary Buat sesi mabar / sparring baru
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

	if _, err := s.courtRepo.FindByID(ctx, req.CourtID); errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("lapangan tidak ditemukan")
	} else if err != nil {
		return helper.Internal(err)
	}

	if err := ValidateSparringCreation(req.Title, req.MaxParticipants, req.Date); err != nil {
		return helper.BadRequest(err.Error())
	}
	if err := ValidateSparringSchedule(req.StartTime, req.EndTime); err != nil {
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

	if _, err := s.bookingRepo.FindSparringByID(ctx, sparringID); errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("sesi sparring tidak ditemukan")
	} else if err != nil {
		return helper.Internal(err)
	}

	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		return s.bookingRepo.JoinSparringAtomic(ctx, tx, sparringID, user.ID)
	})

	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("tidak dapat bergabung: kuota penuh, sesi ditutup, atau Anda sudah terdaftar")
	} else if err != nil {
		var appErr *helper.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "berhasil bergabung ke sesi sparring", nil)
}
