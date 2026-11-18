package service

import (
	"errors"
	"strings"
)

var (
	ErrInvalidRating   = errors.New("rating harus berupa angka antara 1 sampai 5")
	ErrCommentEmpty    = errors.New("komentar minimal 5 karakter")
	ErrSpamDetected    = errors.New("komentar memuat konten terlarang atau spam")
	ErrReplyEmpty      = errors.New("balasan minimal 2 karakter")
	ErrSelfVoteBlocked = errors.New("pembuat ulasan dilarang memberikan upvote pada ulasannya sendiri")
)

var bannedKeywords = []string{"judi", "slot", "promo gratis", "wa.me"}

func ValidateReviewInput(rating int, comment string) error {
	if rating < 1 || rating > 5 {
		return ErrInvalidRating
	}
	trimmed := strings.TrimSpace(comment)
	if len(trimmed) < 5 {
		return ErrCommentEmpty
	}
	lower := strings.ToLower(trimmed)
	for _, kw := range bannedKeywords {
		if strings.Contains(lower, kw) {
			return ErrSpamDetected
		}
	}
	return nil
}

func ValidateReplyInput(comment string) error {
	trimmed := strings.TrimSpace(comment)
	if len(trimmed) < 2 {
		return ErrReplyEmpty
	}
	lower := strings.ToLower(trimmed)
	for _, kw := range bannedKeywords {
		if strings.Contains(lower, kw) {
			return ErrSpamDetected
		}
	}
	return nil
}

func ValidateHelpfulVote(authorUserID, voterUserID int64) error {
	if authorUserID == voterUserID {
		return ErrSelfVoteBlocked
	}
	return nil
}
