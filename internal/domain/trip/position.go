package trip

import (
	"time"

	"github.com/google/uuid"
)

// Position представляет запись о позиции в рамках поездки.
type Position struct {
	ID         string
	TripID     string
	CurPoint   Point
	RecordedAt time.Time
	CreatedAt  time.Time
}

// NewPosition создает новый экземпляр Position с валидацией входных данных.
func NewPosition(tripID string, curPoint Point, recordedAt, now time.Time) (*Position, error) {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "trip_id", value: tripID},
	} {
		parsedID, err := uuid.Parse(field.value)
		if err != nil || parsedID == uuid.Nil {
			return nil, &ValidationError{Field: field.name, Reason: "must be a non-zero UUID"}
		}
	}

	if err := curPoint.validate("current_point"); err != nil {
		return nil, err
	}
	if recordedAt.After(now.Add(time.Minute)) {
		return nil, &ValidationError{Field: "recorded_at", Reason: "is in the future"}
	}

	return &Position{
		TripID:     tripID,
		CurPoint:   curPoint,
		RecordedAt: recordedAt,
		CreatedAt:  now,
	}, nil
}

// RestorePosition собирает позицию из сохраненных данных.
func RestorePosition(
	id, tripID string,
	curPoint Point,
	recordedAt, createdAt time.Time,
) *Position {
	return &Position{
		ID:         id,
		TripID:     tripID,
		CurPoint:   curPoint,
		RecordedAt: recordedAt,
		CreatedAt:  createdAt,
	}
}
