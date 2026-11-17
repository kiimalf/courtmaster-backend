package model

import "time"

type Booking struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	CourtID     int64     `json:"court_id"`
	BookingCode string    `json:"booking_code"`
	Date        string    `json:"date"`
	TotalAmount float64   `json:"total_amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
