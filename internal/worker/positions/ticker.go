package positions

import (
	"fmt"
	"time"
)

type systemTicker struct {
	ticker *time.Ticker
}

// NewTicker creates the production ticker used by the scanner.
func NewTicker(interval time.Duration) (ticker, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("ticker interval must be positive, got %s", interval)
	}

	return systemTicker{ticker: time.NewTicker(interval)}, nil
}

func (t systemTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t systemTicker) Stop() {
	t.ticker.Stop()
}
