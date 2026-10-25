package trip

import (
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

type tripRow struct {
	ID             string
	UserID         string
	DriverID       string
	Status         string
	Price          int64
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// toDomain конвертирует доменную модель tripRow для хранения в базе.
func toDomain(t *trip.Trip) *tripRow {
	return &tripRow{
		ID:             t.ID,
		UserID:         t.UserID,
		DriverID:       t.DriverID,
		Status:         t.Status,
		Price:          t.Price,
		StartLatitude:  t.StartPoint.Latitude,
		StartLongitude: t.StartPoint.Longitude,
		EndLatitude:    t.EndPoint.Latitude,
		EndLongitude:   t.EndPoint.Longitude,
		StartedAt:      t.StartedAt,
		FinishedAt:     t.FinishedAt,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

// fromDomain конвертирует tripRow из базы в доменную модель.
func fromDomain(r *tripRow) *trip.Trip {
	return trip.Restore(r.ID, r.UserID, r.DriverID, r.Status, r.Price,
		trip.Point{Latitude: r.StartLatitude, Longitude: r.StartLongitude},
		trip.Point{Latitude: r.EndLatitude, Longitude: r.EndLongitude},
		r.StartedAt, r.FinishedAt, r.CreatedAt, r.UpdatedAt)
}
