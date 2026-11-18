package service

import (
	"errors"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"

	"github.com/gofiber/fiber/v2"
)

type ReviewService struct {
	repo        repository.ReviewRepository
	bookingRepo repository.BookingRepository
	logger      *slog.Logger
}

func NewReviewService(repo repository.ReviewRepository, bookingRepo repository.BookingRepository, logger *slog.Logger) *ReviewService {
	return &ReviewService{repo: repo, bookingRepo: bookingRepo, logger: logger}
}

// CreateReview godoc
// @Summary Beri ulasan dan rating pada lapangan (Wajib ada riwayat sewa di SQL)
// @Security BearerAuth
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path int true "Court ID"
// @Param request body model.CreateReviewRequest true "Data Review"
// @Success 201 {object} model.WebResponse
// @Router /courts/{id}/reviews [post]
func (s *ReviewService) CreateReview(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courtID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	var req model.CreateReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidateReviewInput(req.Rating, req.Comment); err != nil {
		return helper.BadRequest(err.Error())
	}

	// Verifikasi riwayat sewa di SQL sesuai SOP Bagian 3 & Modul 10
	if s.bookingRepo != nil {
		hasBooked, err := s.bookingRepo.HasUserBookedCourt(ctx, user.ID, courtID)
		if err != nil {
			return helper.Internal(err)
		}
		if !hasBooked {
			return helper.Forbidden("anda hanya dapat memberikan ulasan pada gelanggang yang pernah anda sewa")
		}
	}

	rev := model.CourtReview{
		CourtID:  courtID,
		UserID:   user.ID,
		UserName: user.FullName,
		Rating:   req.Rating,
		Comment:  req.Comment,
		Tags:     req.Tags,
	}

	if err := s.repo.Create(ctx, &rev); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return helper.Conflict("anda sudah memberikan ulasan untuk gelanggang ini")
		}
		return helper.Internal(err)
	}

	return helper.Created(c, "ulasan berhasil ditambahkan", rev, "")
}

// ListReviews godoc
// @Summary Ambil daftar ulasan gelanggang
// @Tags Reviews
// @Produce json
// @Param id path int true "Court ID"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/reviews [get]
func (s *ReviewService) ListReviews(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courtID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	reviews, err := s.repo.FindByCourtID(ctx, courtID)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "daftar ulasan berhasil diambil", reviews)
}

// GetCourtSummary godoc
// @Summary Ringkasan agregasi rating & total ulasan gelanggang
// @Tags Reviews
// @Produce json
// @Param id path int true "Court ID"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/reviews/summary [get]
func (s *ReviewService) GetCourtSummary(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courtID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	summary, err := s.repo.GetCourtSummary(ctx, courtID)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "ringkasan ulasan berhasil diambil", summary)
}

// VoteHelpful godoc
// @Summary Beri upvote bermanfaat pada ulasan (Anti-Self-Vote)
// @Security BearerAuth
// @Tags Reviews
// @Param id path string true "Review ID"
// @Success 200 {object} model.WebResponse
// @Router /reviews/{id}/helpful [post]
func (s *ReviewService) VoteHelpful(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	reviewID := c.Params("reviewId")
	if reviewID == "" {
		reviewID = c.Params("id")
	}

	rev, err := s.repo.FindByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("ulasan tidak ditemukan")
		}
		return helper.Internal(err)
	}

	if err := ValidateHelpfulVote(rev.UserID, user.ID); err != nil {
		return helper.BadRequest(err.Error())
	}

	if err := s.repo.VoteHelpful(ctx, reviewID, user.ID); err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "vote bermanfaat berhasil dicatat", nil)
}

// AddReply godoc
// @Summary Tambah balasan pada ulasan ($push atomik maksimal 50 balasan)
// @Security BearerAuth
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID"
// @Param request body model.AddReplyRequest true "Balasan Ulasan"
// @Success 200 {object} model.WebResponse
// @Router /reviews/{id}/replies [post]
func (s *ReviewService) AddReply(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	reviewID := c.Params("reviewId")
	if reviewID == "" {
		reviewID = c.Params("id")
	}

	var req model.AddReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidateReplyInput(req.Comment); err != nil {
		return helper.BadRequest(err.Error())
	}

	reply := model.ReviewReply{
		UserID:    user.ID,
		UserName:  user.FullName,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
	}

	if err := s.repo.AddReply(ctx, reviewID, reply); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("ulasan tidak ditemukan")
		}
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "balasan berhasil ditambahkan", reply)
}
