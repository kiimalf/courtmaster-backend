package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"courtmaster-backend/app/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.CourtReview) error
	FindByID(ctx context.Context, reviewID string) (*model.CourtReview, error)
	FindByCourtID(ctx context.Context, courtID int64) ([]model.CourtReview, error)
	AddReply(ctx context.Context, reviewID string, reply model.ReviewReply) error
	VoteHelpful(ctx context.Context, reviewID string, userID int64) error
	GetCourtSummary(ctx context.Context, courtID int64) (*model.ReviewSummary, error)
}

type reviewMongoRepo struct {
	col *mongo.Collection
}

type reviewDoc struct {
	ID        bson.ObjectID       `bson:"_id,omitempty"`
	CourtID   int64               `bson:"court_id"`
	UserID    int64               `bson:"user_id"`
	UserName  string              `bson:"user_name"`
	Rating    int                 `bson:"rating"`
	Comment   string              `bson:"comment"`
	Tags      []string            `bson:"tags"`
	HelpfulBy []int64             `bson:"helpful_by"`
	Replies   []model.ReviewReply `bson:"replies"`
	CreatedAt time.Time           `bson:"created_at"`
}

func NewReviewRepository(db *mongo.Database) ReviewRepository {
	col := db.Collection("court_reviews")
	return &reviewMongoRepo{col: col}
}

func (r *reviewMongoRepo) Create(ctx context.Context, rev *model.CourtReview) error {
	doc := reviewDoc{
		CourtID:   rev.CourtID,
		UserID:    rev.UserID,
		UserName:  rev.UserName,
		Rating:    rev.Rating,
		Comment:   rev.Comment,
		Tags:      rev.Tags,
		HelpfulBy: []int64{},
		Replies:   []model.ReviewReply{},
		CreatedAt: time.Now(),
	}

	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("reviewRepo.Create: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		rev.ID = oid.Hex()
		rev.CreatedAt = doc.CreatedAt
	}
	return nil
}

func (r *reviewMongoRepo) FindByID(ctx context.Context, reviewID string) (*model.CourtReview, error) {
	objID, err := bson.ObjectIDFromHex(reviewID)
	if err != nil {
		return nil, ErrNotFound
	}
	var doc reviewDoc
	err = r.col.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &model.CourtReview{
		ID:        doc.ID.Hex(),
		CourtID:   doc.CourtID,
		UserID:    doc.UserID,
		UserName:  doc.UserName,
		Rating:    doc.Rating,
		Comment:   doc.Comment,
		Tags:      doc.Tags,
		HelpfulBy: doc.HelpfulBy,
		Replies:   doc.Replies,
		CreatedAt: doc.CreatedAt,
	}, nil
}

func (r *reviewMongoRepo) FindByCourtID(ctx context.Context, courtID int64) ([]model.CourtReview, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{"court_id": courtID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []reviewDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	reviews := make([]model.CourtReview, 0, len(docs))
	for _, d := range docs {
		reviews = append(reviews, model.CourtReview{
			ID:        d.ID.Hex(),
			CourtID:   d.CourtID,
			UserID:    d.UserID,
			UserName:  d.UserName,
			Rating:    d.Rating,
			Comment:   d.Comment,
			Tags:      d.Tags,
			HelpfulBy: d.HelpfulBy,
			Replies:   d.Replies,
			CreatedAt: d.CreatedAt,
		})
	}
	return reviews, nil
}

func (r *reviewMongoRepo) AddReply(ctx context.Context, reviewID string, reply model.ReviewReply) error {
	objID, err := bson.ObjectIDFromHex(reviewID)
	if err != nil {
		return err
	}
	update := bson.M{
		"$push": bson.M{
			"replies": bson.M{
				"$each":  []model.ReviewReply{reply},
				"$slice": -50,
			},
		},
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": objID}, update)
	return err
}

func (r *reviewMongoRepo) VoteHelpful(ctx context.Context, reviewID string, userID int64) error {
	objID, err := bson.ObjectIDFromHex(reviewID)
	if err != nil {
		return err
	}
	update := bson.M{
		"$addToSet": bson.M{"helpful_by": userID},
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": objID}, update)
	return err
}

func (r *reviewMongoRepo) GetCourtSummary(ctx context.Context, courtID int64) (*model.ReviewSummary, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"court_id": courtID}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":           nil,
			"avg_rating":    bson.M{"$avg": "$rating"},
			"total_reviews": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		AvgRating    float64 `bson:"avg_rating"`
		TotalReviews int     `bson:"total_reviews"`
	}
	if err := cursor.All(ctx, &results); err != nil || len(results) == 0 {
		return &model.ReviewSummary{AverageRating: 0, TotalReviews: 0}, nil
	}

	return &model.ReviewSummary{
		AverageRating: results[0].AvgRating,
		TotalReviews:  results[0].TotalReviews,
	}, nil
}
