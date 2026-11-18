package service

import (
	"errors"
	"testing"
	"time"
)

func testDates() (tomorrow, yesterday string) {
	return time.Now().Add(24 * time.Hour).Format(dateLayout), time.Now().Add(-24 * time.Hour).Format(dateLayout)
}

func TestValidateBookingRequest(t *testing.T) {
	tomorrow, yesterday := testDates()

	tests := []struct {
		name         string
		date         string
		slotIDs      []int64
		currentHours int
		wantErr      error
	}{
		{"Valid 2 Hours", tomorrow, []int64{1, 2}, 0, nil},
		{"Valid Remaining Quota", tomorrow, []int64{1, 2}, 2, nil},
		{"Past Date", yesterday, []int64{1}, 0, ErrPastDate},
		{"Invalid Date Format", "18-11-2026", []int64{1}, 0, ErrInvalidDate},
		{"Zero Slots", tomorrow, []int64{}, 0, ErrEmptySlots},
		{"Nil Slots", tomorrow, nil, 0, ErrEmptySlots},
		{"More than 4 Slots", tomorrow, []int64{1, 2, 3, 4, 5}, 0, ErrTooManySlots},
		{"Duplicate Slots", tomorrow, []int64{1, 2, 1}, 0, ErrDuplicateSlots},
		{"Exceed 4 Hours Daily Limit", tomorrow, []int64{1, 2, 3}, 2, ErrExceedDailyLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBookingRequest(tt.date, tt.slotIDs, tt.currentHours)
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
	tomorrow, yesterday := testDates()

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
		{"Invalid Date Swap", "2026/11/18", 1, 2, ErrInvalidDate},
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
	tomorrow, yesterday := testDates()

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
		{"Invalid Date Sparring", "Mabar Tennis", 4, "kemarin", ErrInvalidDate},
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

func TestValidateSparringSchedule(t *testing.T) {
	tests := []struct {
		name      string
		startTime string
		endTime   string
		wantErr   error
	}{
		{"Valid Schedule", "18:00", "20:00", nil},
		{"Equal Times", "18:00", "18:00", ErrInvalidTimeRange},
		{"End Before Start", "20:00", "18:00", ErrInvalidTimeRange},
		{"Invalid Start Format", "pukul 6", "20:00", ErrInvalidDate},
		{"Invalid End Format", "18:00", "8 malam", ErrInvalidDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSparringSchedule(tt.startTime, tt.endTime)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}
