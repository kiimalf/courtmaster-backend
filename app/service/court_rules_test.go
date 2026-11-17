package service

import (
	"errors"
	"testing"
)

func TestValidateCourtCreation(t *testing.T) {
	tests := []struct {
		name       string
		courtName  string
		sportType  string
		hourlyRate float64
		wantErr    error
	}{
		{"Valid Padel Court", "Lap A Padel", "padel", 150000, nil},
		{"Valid Badminton", "Court 1", "badminton", 60000, nil},
		{"Empty Name", "", "padel", 100000, ErrEmptyCourtName},
		{"Invalid Sport", "Lap 2", "tennis_meja", 50000, ErrInvalidSportType},
		{"Zero Rate", "Lap 3", "futsal", 0, ErrInvalidRate},
		{"Negative Rate", "Lap 4", "basketball", -5000, ErrInvalidRate},
		{"Exceeded Max Rate", "VIP Court", "padel", 2500000, ErrMaxRateExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCourtCreation(tt.courtName, tt.sportType, tt.hourlyRate)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}
