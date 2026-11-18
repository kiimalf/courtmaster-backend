package service

import (
	"errors"
	"testing"
)

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"Valid Password", "Rahasia123", nil},
		{"Too Short", "Pass1", ErrPasswordTooShort},
		{"No Number", "HanyaHurufSaja", ErrPasswordNoNumber},
		{"No Letter", "1234567890", ErrPasswordNoLetter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || err.Error() != tt.wantErr.Error()) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateRoleChange(t *testing.T) {
	err := ValidateRoleChange(1, 1, "admin", "player")
	if !errors.Is(err, ErrSelfDemote) {
		t.Fatalf("expected ErrSelfDemote, got: %v", err)
	}

	err = ValidateRoleChange(1, 2, "admin", "player")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}
