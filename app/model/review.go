package model

import "time"

type CourtReview struct {
	ID        string        `json:"id" bson:"_id,omitempty"`
	CourtID   int64         `json:"court_id" bson:"court_id"`
	UserID    int64         `json:"user_id" bson:"user_id"`
	UserName  string        `json:"user_name" bson:"user_name"`
	Rating    int           `json:"rating" bson:"rating"` // 1 s.d. 5
	Comment   string        `json:"comment" bson:"comment"`
	Tags      []string      `json:"tags" bson:"tags"`
	HelpfulBy []int64       `json:"helpful_by" bson:"helpful_by"`
	Replies   []ReviewReply `json:"replies" bson:"replies"`
	CreatedAt time.Time     `json:"created_at" bson:"created_at"`
}

type ReviewReply struct {
	UserID    int64     `json:"user_id" bson:"user_id"`
	UserName  string    `json:"user_name" bson:"user_name"`
	Comment   string    `json:"comment" bson:"comment"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type CreateReviewRequest struct {
	Rating  int      `json:"rating" validate:"required,min=1,max=5"`
	Comment string   `json:"comment" validate:"required,min=5,max=500"`
	Tags    []string `json:"tags" validate:"omitempty,max=5"`
}

type AddReplyRequest struct {
	Comment string `json:"comment" validate:"required,min=2,max=300"`
}

type ReviewSummary struct {
	AverageRating float64 `json:"average_rating"`
	TotalReviews  int     `json:"total_reviews"`
}
