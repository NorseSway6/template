package positions

import (
	"fmt"
	"time"
)

type systemTicker struct {
	ticker *time.Ticker
}

// NewTicker создает новый тикер с указанным интервалом.
func NewTicker(interval time.Duration) (ticker, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("ticker interval must be positive, got %s", interval)
	}

	return systemTicker{ticker: time.NewTicker(interval)}, nil
}

// C возвращает канал, на который будут отправляться события тикера.
func (t systemTicker) C() <-chan time.Time {
	return t.ticker.C
}

// Stop останавливает тикер.
func (t systemTicker) Stop() {
	t.ticker.Stop()
}
