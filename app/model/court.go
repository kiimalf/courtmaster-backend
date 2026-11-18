package model

import "time"

type Court struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	SportType  string    `json:"sport_type"`
	HourlyRate float64   `json:"hourly_rate"`
	Version    int       `json:"version"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

type CourtSlot struct {
	ID          int64  `json:"id"`
	CourtID     int64  `json:"court_id"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	IsAvailable bool   `json:"is_available"`
	BookingID   *int64 `json:"booking_id,omitempty"`
	Version     int    `json:"version"`
}

type CreateCourtRequest struct {
	Name       string  `json:"name" validate:"required,min=3,max=100"`
	SportType  string  `json:"sport_type" validate:"required,oneof=padel badminton futsal basketball"`
	HourlyRate float64 `json:"hourly_rate" validate:"required,gt=0"`
}

type UpdateHourlyRateRequest struct {
	HourlyRate float64 `json:"hourly_rate" validate:"required,gt=0,lte=2000000"`
	Version    int     `json:"version" validate:"required,gt=0"`
}
