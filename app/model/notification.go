package model

import (
	"encoding/json"
	"time"
)

const EventTypeBookingCreated = "booking:created"

type OutboxEvent struct {
	ID          int64           `json:"id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at"`
}

type NotificationJob struct {
	OutboxID  int64           `json:"outbox_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type Notification struct {
	ID       int64     `json:"id"`
	OutboxID int64     `json:"outbox_id"`
	UserID   int64     `json:"user_id"`
	Channel  string    `json:"channel"`
	Subject  string    `json:"subject"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
}

type BookingCreatedPayload struct {
	BookingID   int64   `json:"booking_id"`
	BookingCode string  `json:"booking_code"`
	UserID      int64   `json:"user_id"`
	CourtID     int64   `json:"court_id"`
	Date        string  `json:"date"`
	TotalAmount float64 `json:"total_amount"`
}
