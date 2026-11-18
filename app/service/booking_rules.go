package service

import (
	"errors"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

var (
	ErrInvalidDate         = errors.New("format tanggal tidak valid, gunakan YYYY-MM-DD")
	ErrPastDate            = errors.New("tanggal pemesanan tidak boleh di masa lampau")
	ErrExceedDailyLimit    = errors.New("total pemesanan melebihi batas kuota 4 jam per hari")
	ErrEmptySlots          = errors.New("minimal memilih 1 slot jam")
	ErrTooManySlots        = errors.New("maksimal memilih 4 slot jam per transaksi")
	ErrDuplicateSlots      = errors.New("slot jam tidak boleh duplikat dalam satu transaksi")
	ErrSameSlotSwap        = errors.New("slot baru tidak boleh sama dengan slot lama")
	ErrInvalidParticipants = errors.New("jumlah peserta sparring minimal 2 orang")
	ErrEmptyTitle          = errors.New("judul sparring tidak boleh kosong")
	ErrInvalidTimeRange    = errors.New("waktu selesai sparring harus setelah waktu mulai")
)

// todayString mengembalikan tanggal hari ini dalam format YYYY-MM-DD.
func todayString() string {
	return time.Now().Format(dateLayout)
}

// ValidateBookingRequest memvalidasi tanggal, jumlah slot, duplikasi slot, dan
// batas kuota harian 4 jam (Pure Logic, tanpa Fiber maupun database).
func ValidateBookingRequest(bookingDateStr string, slotIDs []int64, existingBookedHours int) error {
	if _, err := time.Parse(dateLayout, bookingDateStr); err != nil {
		return ErrInvalidDate
	}

	if bookingDateStr < todayString() {
		return ErrPastDate
	}

	slotCount := len(slotIDs)
	if slotCount < 1 {
		return ErrEmptySlots
	}
	if slotCount > 4 {
		return ErrTooManySlots
	}

	seen := make(map[int64]struct{}, slotCount)
	for _, id := range slotIDs {
		if _, exists := seen[id]; exists {
			return ErrDuplicateSlots
		}
		seen[id] = struct{}{}
	}

	if existingBookedHours+slotCount > 4 {
		return ErrExceedDailyLimit
	}

	return nil
}

// ValidateSwapRequest memvalidasi integritas penukaran slot jam.
func ValidateSwapRequest(bookingDateStr string, oldSlotID, newSlotID int64) error {
	if oldSlotID == newSlotID {
		return ErrSameSlotSwap
	}

	if _, err := time.Parse(dateLayout, bookingDateStr); err != nil {
		return ErrInvalidDate
	}

	if bookingDateStr < todayString() {
		return ErrPastDate
	}

	return nil
}

// ValidateSparringCreation memvalidasi aturan pembuatan sesi mabar sparring.
func ValidateSparringCreation(title string, maxParticipants int, dateStr string) error {
	if strings.TrimSpace(title) == "" {
		return ErrEmptyTitle
	}
	if maxParticipants < 2 {
		return ErrInvalidParticipants
	}
	if _, err := time.Parse(dateLayout, dateStr); err != nil {
		return ErrInvalidDate
	}
	if dateStr < todayString() {
		return ErrPastDate
	}
	return nil
}

// ValidateSparringSchedule memastikan rentang jam sesi mabar valid (HH:MM).
func ValidateSparringSchedule(startStr, endStr string) error {
	start, err := time.Parse("15:04", startStr)
	if err != nil {
		return ErrInvalidDate
	}
	end, err := time.Parse("15:04", endStr)
	if err != nil {
		return ErrInvalidDate
	}
	if !end.After(start) {
		return ErrInvalidTimeRange
	}
	return nil
}
