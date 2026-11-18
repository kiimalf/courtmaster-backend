package service

import (
	"errors"
	"unicode"
)

var (
	ErrPasswordTooShort = errors.New("password minimal 8 karakter")
	ErrPasswordNoNumber = errors.New("password wajib memuat minimal 1 angka")
	ErrPasswordNoLetter = errors.New("password wajib memuat minimal 1 huruf")
	ErrSelfDemote       = errors.New("admin dilarang menurunkan role akunnya sendiri")
)

func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	var hasNumber, hasLetter bool
	for _, char := range password {
		if unicode.IsDigit(char) {
			hasNumber = true
		}
		if unicode.IsLetter(char) {
			hasLetter = true
		}
	}
	if !hasLetter {
		return ErrPasswordNoLetter
	}
	if !hasNumber {
		return ErrPasswordNoNumber
	}
	return nil
}

func ValidateRoleChange(currentUserID int64, targetUserID int64, currentRole string, targetRole string) error {
	if currentUserID == targetUserID && currentRole == "admin" && targetRole != "admin" {
		return ErrSelfDemote
	}
	return nil
}
