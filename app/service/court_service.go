package service

import (
	"errors"
	"log/slog"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"

	"github.com/gofiber/fiber/v2"
)

type CourtService struct {
	repo   repository.CourtRepository
	logger *slog.Logger
}

func NewCourtService(repo repository.CourtRepository, logger *slog.Logger) *CourtService {
	return &CourtService{repo: repo, logger: logger}
}

// ListCourts godoc
// @Summary Daftar seluruh gelanggang olahraga
// @Tags Courts
// @Produce json
// @Param sport_type query string false "Filter jenis olahraga"
// @Success 200 {object} model.WebResponse
// @Router /courts [get]
func (s *CourtService) ListCourts(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	sportType := c.Query("sport_type")
	courts, err := s.repo.FindAll(ctx, sportType)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar lapangan berhasil diambil", courts)
}

// GetCourtByID godoc
// @Summary Ambil detail lapangan berdasarkan ID
// @Tags Courts
// @Produce json
// @Param id path int true "Court ID"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id} [get]
func (s *CourtService) GetCourtByID(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	court, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("lapangan tidak ditemukan")
	} else if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "detail lapangan berhasil diambil", court)
}

// ListSlots godoc
// @Summary Ambil jadwal slot jam ketersediaan lapangan
// @Tags Courts
// @Produce json
// @Param id path int true "Court ID"
// @Param date query string true "Tanggal format YYYY-MM-DD"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/slots [get]
func (s *CourtService) ListSlots(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}
	date := c.Query("date")
	if date == "" {
		return helper.BadRequest("parameter date wajib diisi (YYYY-MM-DD)")
	}

	slots, err := s.repo.FindSlotsByCourtAndDate(ctx, id, date)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "jadwal slot lapangan berhasil diambil", slots)
}

// CreateCourt godoc
// @Summary Tambah gelanggang baru (Admin/Manager)
// @Security BearerAuth
// @Tags Courts
// @Accept json
// @Produce json
// @Param request body model.CreateCourtRequest true "Data Lapangan"
// @Success 201 {object} model.WebResponse
// @Router /courts [post]
func (s *CourtService) CreateCourt(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateCourtRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("format body JSON tidak valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidateCourtCreation(req.Name, req.SportType, req.HourlyRate); err != nil {
		return helper.BadRequest(err.Error())
	}

	court := model.Court{
		Name:       req.Name,
		SportType:  req.SportType,
		HourlyRate: req.HourlyRate,
	}
	if err := s.repo.Create(ctx, &court); err != nil {
		return helper.Internal(err)
	}
	return helper.Created(c, "lapangan berhasil ditambahkan", court, "")
}

// UpdateHourlyRate godoc
// @Summary Update tarif per jam dengan Optimistic Locking
// @Security BearerAuth
// @Tags Courts
// @Accept json
// @Produce json
// @Param id path int true "Court ID"
// @Param request body model.UpdateHourlyRateRequest true "Update Tarif"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/rate [patch]
func (s *CourtService) UpdateHourlyRate(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}
	var req model.UpdateHourlyRateRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("format body JSON tidak valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	err := s.repo.UpdateHourlyRateOptimistic(ctx, id, req.HourlyRate, req.Version)
	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("gagal update tarif: data lapangan telah dimodifikasi oleh pengguna lain, silakan refresh")
	} else if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "tarif lapangan berhasil diupdate", nil)
}
