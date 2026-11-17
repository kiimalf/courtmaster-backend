package service

import (
	"errors"
	"strings"
)

var (
	ErrEmptyCourtName   = errors.New("nama lapangan tidak boleh kosong")
	ErrInvalidSportType = errors.New("jenis olahraga tidak valid, pilih: padel, badminton, futsal, atau basketball")
	ErrInvalidRate      = errors.New("tarif sewa per jam harus lebih besar dari 0")
	ErrMaxRateExceeded  = errors.New("tarif sewa per jam tidak boleh melebihi Rp 2.000.000")
)

var allowedSports = map[string]bool{
	"padel":      true,
	"badminton":  true,
	"futsal":     true,
	"basketball": true,
}

func ValidateCourtCreation(name string, sportType string, hourlyRate float64) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyCourtName
	}
	sport := strings.ToLower(strings.TrimSpace(sportType))
	if !allowedSports[sport] {
		return ErrInvalidSportType
	}
	if hourlyRate <= 0 {
		return ErrInvalidRate
	}
	if hourlyRate > 2_000_000 {
		return ErrMaxRateExceeded
	}
	return nil
}
