package trip

import "time"

// Блок констант со статусами поездки.
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

// Point представляет структуру координат.
type Point struct {
	Latitude  float64
	Longitude float64
}

// Trip представляет модель данных для таблицы trips.
type Trip struct {
	ID         string
	UserID     string
	DriverID   string
	Status     string
	Price      int64
	StartPoint Point
	EndPoint   Point
	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// New создает новый экземпляр Trip с валидацией входных данных.
func New(id, userId, driverId string, price int64, startPoint, endPoint Point, startedAt time.Time) (*Trip, error) {
	if price < 0 {
		return nil, &ValidationError{Field: "price", Reason: "must be >= 0"}
	}
	if err := startPoint.validate("start_point"); err != nil {
		return nil, err
	}
	if err := endPoint.validate("end_point"); err != nil {
		return nil, err
	}

	return &Trip{
		ID:         id,
		UserID:     userId,
		DriverID:   driverId,
		Status:     StatusActive,
		Price:      price,
		StartPoint: startPoint,
		EndPoint:   endPoint,
		StartedAt:  startedAt,
	}, nil
}

// Restore собирает поездку из сохраненных данных.
func Restore(
	id, userID, driverID, status string,
	price int64,
	startPoint, endPoint Point,
	startedAt time.Time, finishedAt *time.Time,
	createdAt, updatedAt time.Time,
) *Trip {
	return &Trip{
		ID: id, UserID: userID, DriverID: driverID, Status: status, Price: price,
		StartPoint: startPoint, EndPoint: endPoint,
		StartedAt: startedAt, FinishedAt: finishedAt,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

// Finish завершает поездку, устанавливая статус в completed и фиксируя время завершения.
func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}
	t.Status = StatusCompleted
	t.FinishedAt = &at
	t.UpdatedAt = at
	return nil
}

// validate проверяет корректность координат точки.
func (p Point) validate(field string) error {
	if p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
		return &ValidationError{Field: field, Reason: "coordinates out of bounds"}
	}
	return nil
}
