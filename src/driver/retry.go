package driver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines retry behavior with exponential backoff and jitter.
type RetryPolicy struct {
	MaxAttempts  int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	JitterFactor float64 // 0.0 = no jitter, 1.0 = full jitter

	// Callbacks for observability
	OnRetry   func(ctx RetryContext)
	OnSuccess func(attempts int)
	OnFailure func(err error, attempts int)
}

// RetryContext provides context to retry callbacks.
type RetryContext struct {
	Attempt         int
	Error           error
	NextDelay       time.Duration
	CumulativeDelay time.Duration
}

// RetryError wraps the original error with retry context.
type RetryError struct {
	OriginalError   error
	Attempts        int
	CumulativeDelay time.Duration
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("max retries (%d) exceeded after %v: %v",
		e.Attempts, e.CumulativeDelay, e.OriginalError)
}

func (e *RetryError) Unwrap() error {
	return e.OriginalError
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  5,
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 1.0, // Full jitter by default
	}
}

// NoRetryPolicy returns a policy that doesn't retry.
func NoRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: 1,
	}
}

// CalculateDelay computes the delay for a given attempt using exponential backoff with jitter.
// Uses the "full jitter" algorithm to prevent thundering herd.
func (p *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	// Exponential: base * multiplier^(attempt-1)
	exponent := float64(attempt - 1)
	baseExp := float64(p.BaseDelay) * math.Pow(p.Multiplier, exponent)

	// Cap at max delay
	capped := math.Min(baseExp, float64(p.MaxDelay))

	// Apply jitter: delay * (1 - jitter + rand * jitter)
	jitter := math.Max(0, math.Min(1, p.JitterFactor))
	randomScalar := rand.Float64()
	jitterBlend := 1.0 - jitter + randomScalar*jitter
	jittered := capped * jitterBlend

	return time.Duration(jittered)
}

// IsRetriable checks if an error should trigger a retry.
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}

	// Check for DatabaseError
	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.IsRetriable()
	}

	// Check for context errors (not retriable)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network/connection errors are generally retriable
	errMsg := err.Error()
	if contains(errMsg, "connection refused", "connection reset", "broken pipe",
		"EOF", "timeout", "temporary failure") {
		return true
	}

	return false
}

// contains checks if s contains any of the substrings (case-insensitive).
func contains(s string, substrs ...string) bool {
	lower := s
	for _, sub := range substrs {
		if len(sub) > 0 && len(lower) >= len(sub) {
			for i := 0; i <= len(lower)-len(sub); i++ {
				match := true
				for j := 0; j < len(sub); j++ {
					c1, c2 := lower[i+j], sub[j]
					if c1 >= 'A' && c1 <= 'Z' {
						c1 += 32
					}
					if c2 >= 'A' && c2 <= 'Z' {
						c2 += 32
					}
					if c1 != c2 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

// Retry executes fn with retry logic according to the policy.
func Retry[T any](ctx context.Context, policy *RetryPolicy, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	var cumulativeDelay time.Duration

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			if policy.OnSuccess != nil {
				policy.OnSuccess(attempt)
			}
			return result, nil
		}

		lastErr = err

		// Check if retriable
		if !IsRetriable(err) {
			if policy.OnFailure != nil {
				policy.OnFailure(err, attempt)
			}
			return zero, err
		}

		// Check if we've exhausted retries
		if attempt >= policy.MaxAttempts {
			break
		}

		// Calculate delay
		delay := policy.CalculateDelay(attempt)
		cumulativeDelay += delay

		// Callback before sleep
		if policy.OnRetry != nil {
			policy.OnRetry(RetryContext{
				Attempt:         attempt,
				Error:           err,
				NextDelay:       delay,
				CumulativeDelay: cumulativeDelay,
			})
		}

		// Sleep with context awareness
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	// Exhausted retries
	if policy.OnFailure != nil {
		policy.OnFailure(lastErr, policy.MaxAttempts)
	}

	return zero, &RetryError{
		OriginalError:   lastErr,
		Attempts:        policy.MaxAttempts,
		CumulativeDelay: cumulativeDelay,
	}
}

// RetryVoid executes fn with retry logic for functions that don't return a value.
func RetryVoid(ctx context.Context, policy *RetryPolicy, fn func() error) error {
	_, err := Retry(ctx, policy, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}
