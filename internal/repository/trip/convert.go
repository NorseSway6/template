package trip

import (
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// tripRow представляет собой структуру для хранения данных о поездке в базе данных.
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
	LastPosAt      *time.Time
}

// positionRow представляет собой структуру для хранения данных о позиции в базе данных.
type positionRow struct {
	ID         string
	TripID     string
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	CreatedAt  time.Time
}

// fromDomain конвертирует доменную модель tripRow для хранения в базе.
func fromDomain(t *trip.Trip) *tripRow {
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
		LastPosAt:      t.LastPositionAt,
	}
}

// toDomain конвертирует tripRow из базы в доменную модель.
func toDomain(r *tripRow) *trip.Trip {
	return trip.Restore(r.ID, r.UserID, r.DriverID, r.Status, r.Price,
		trip.Point{Latitude: r.StartLatitude, Longitude: r.StartLongitude},
		trip.Point{Latitude: r.EndLatitude, Longitude: r.EndLongitude},
		r.StartedAt, r.FinishedAt, r.LastPosAt, r.CreatedAt, r.UpdatedAt)
}

// fromDomainPosition конвертирует доменную модель positionRow для хранения в базе.
func fromDomainPosition(p *trip.Position) *positionRow {
	return &positionRow{
		ID:         p.ID,
		TripID:     p.TripID,
		Latitude:   p.CurPoint.Latitude,
		Longitude:  p.CurPoint.Longitude,
		RecordedAt: p.RecordedAt,
		CreatedAt:  p.CreatedAt,
	}
}

// toDomainPosition конвертирует positionRow из базы в доменную модель.
func toDomainPosition(r *positionRow) *trip.Position {
	return trip.RestorePosition(r.ID, r.TripID,
		trip.Point{Latitude: r.Latitude, Longitude: r.Longitude},
		r.RecordedAt, r.CreatedAt)
}
