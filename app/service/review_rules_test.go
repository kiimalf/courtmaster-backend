package service

import (
	"errors"
	"testing"
)

func TestValidateReviewInput(t *testing.T) {
	tests := []struct {
		name    string
		rating  int
		comment string
		wantErr error
	}{
		{"Valid Review", 5, "Lapangan sangat bagus dan bersih!", nil},
		{"Rating 0", 0, "Kurang memuaskan", ErrInvalidRating},
		{"Rating 6", 6, "Sangat luar biasa", ErrInvalidRating},
		{"Short Comment", 4, "ok", ErrCommentEmpty},
		{"Spam Content", 5, "Daftar slot online wa.me/123", ErrSpamDetected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReviewInput(tt.rating, tt.comment)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateHelpfulVote(t *testing.T) {
	if err := ValidateHelpfulVote(10, 10); !errors.Is(err, ErrSelfVoteBlocked) {
		t.Fatalf("expected ErrSelfVoteBlocked, got: %v", err)
	}
	if err := ValidateHelpfulVote(10, 20); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}
