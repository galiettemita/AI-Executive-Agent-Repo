package connectors

import "time"

type RetryPolicy struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      time.Duration
	MaxAttempts int
}

type CircuitBreakerPolicy struct {
	FailureThreshold int
	Window           time.Duration
	HalfOpenAfter    time.Duration
}

type TimeoutPolicy struct {
	Default time.Duration
	FileOps time.Duration
	Health  time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		BaseDelay:   1 * time.Second,
		MaxDelay:    60 * time.Second,
		Jitter:      500 * time.Millisecond,
		MaxAttempts: 5,
	}
}

func DefaultCircuitBreakerPolicy() CircuitBreakerPolicy {
	return CircuitBreakerPolicy{
		FailureThreshold: 5,
		Window:           60 * time.Second,
		HalfOpenAfter:    300 * time.Second,
	}
}

func DefaultTimeoutPolicy() TimeoutPolicy {
	return TimeoutPolicy{
		Default: 30 * time.Second,
		FileOps: 120 * time.Second,
		Health:  5 * time.Second,
	}
}
