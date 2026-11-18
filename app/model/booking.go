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

type CreateBookingRequest struct {
	CourtID int64   `json:"court_id" validate:"required,gt=0"`
	Date    string  `json:"date" validate:"required,datetime=2006-01-02"`
	SlotIDs []int64 `json:"slot_ids" validate:"required,min=1,max=4,dive,gt=0"`
}

type SwapBookingRequest struct {
	BookingID int64  `json:"booking_id" validate:"required,gt=0"`
	CourtID   int64  `json:"court_id" validate:"required,gt=0"`
	Date      string `json:"date" validate:"required,datetime=2006-01-02"`
	OldSlotID int64  `json:"old_slot_id" validate:"required,gt=0"`
	NewSlotID int64  `json:"new_slot_id" validate:"required,gt=0"`
}

type Sparring struct {
	ID                  int64     `json:"id"`
	HostID              int64     `json:"host_id"`
	CourtID             int64     `json:"court_id"`
	Title               string    `json:"title"`
	Date                string    `json:"date"`
	StartTime           string    `json:"start_time"`
	EndTime             string    `json:"end_time"`
	MaxParticipants     int       `json:"max_participants"`
	CurrentParticipants int       `json:"current_participants"`
	PricePerPerson      float64   `json:"price_per_person"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
}

type CreateSparringRequest struct {
	CourtID         int64   `json:"court_id" validate:"required,gt=0"`
	Title           string  `json:"title" validate:"required,min=5,max=150"`
	Date            string  `json:"date" validate:"required,datetime=2006-01-02"`
	StartTime       string  `json:"start_time" validate:"required,datetime=15:04"`
	EndTime         string  `json:"end_time" validate:"required,datetime=15:04"`
	MaxParticipants int     `json:"max_participants" validate:"required,min=2,max=20"`
	PricePerPerson  float64 `json:"price_per_person" validate:"required,gte=0"`
}
