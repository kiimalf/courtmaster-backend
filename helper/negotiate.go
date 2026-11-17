package helper

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"courtmaster-backend/app/model"

	"github.com/gofiber/fiber/v2"
)

const (
	FormatJSON = fiber.MIMEApplicationJSON
	FormatCSV  = "text/csv"
)

func Negotiate(c *fiber.Ctx, offered ...string) (string, error) {
	accept := strings.TrimSpace(c.Get(fiber.HeaderAccept))
	if accept == "" {
		return offered[0], nil
	}
	chosen := c.Accepts(offered...)
	if chosen == "" {
		return "", NotAcceptable("format yang diminta tidak didukung, pilih: " + strings.Join(offered, ", "))
	}
	return chosen, nil
}

func WriteBookingsCSV(c *fiber.Ctx, bookings []model.Booking) error {
	c.Set(fiber.HeaderContentType, FormatCSV+"; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="bookings.csv"`)

	var buffer strings.Builder
	writer := csv.NewWriter(&buffer)

	header := []string{"id", "booking_code", "court_id", "date", "total_amount", "status", "created_at"}
	if err := writer.Write(header); err != nil {
		return Internal(err)
	}

	for _, b := range bookings {
		row := []string{
			strconv.FormatInt(b.ID, 10),
			b.BookingCode,
			strconv.FormatInt(b.CourtID, 10),
			b.Date,
			fmt.Sprintf("%.2f", b.TotalAmount),
			b.Status,
			b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if err := writer.Write(row); err != nil {
			return Internal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return Internal(err)
	}
	return c.SendString(buffer.String())
}
