package integration

import (
	"fmt"
	"time"
)

type ChannelIngressRetryResult struct {
	Attempts      int
	DeadLettered  bool
	BackoffDelays []time.Duration
}

func IngressBackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	return time.Duration(1<<(attempt-1)) * time.Second
}

func ProcessChannelIngressWithRetry(maxRetries int, attemptFn func(attempt int) error) (ChannelIngressRetryResult, error) {
	if attemptFn == nil {
		return ChannelIngressRetryResult{}, fmt.Errorf("attempt function is required")
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}

	delays := make([]time.Duration, 0, maxRetries)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := attemptFn(attempt); err == nil {
			return ChannelIngressRetryResult{
				Attempts:      attempt,
				DeadLettered:  false,
				BackoffDelays: delays,
			}, nil
		}
		delays = append(delays, IngressBackoffDelay(attempt))
	}

	return ChannelIngressRetryResult{
		Attempts:      maxRetries,
		DeadLettered:  true,
		BackoffDelays: delays,
	}, nil
}
