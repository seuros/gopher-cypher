package driver

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := &RetryPolicy{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0, // No jitter for predictable tests
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},  // base * 2^0 = 100ms
		{2, 200 * time.Millisecond},  // base * 2^1 = 200ms
		{3, 400 * time.Millisecond},  // base * 2^2 = 400ms
		{4, 800 * time.Millisecond},  // base * 2^3 = 800ms
		{5, 1600 * time.Millisecond}, // base * 2^4 = 1600ms
	}

	for _, tt := range tests {
		delay := policy.CalculateDelay(tt.attempt)
		if delay != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, delay)
		}
	}
}

func TestRetryPolicy_CalculateDelay_MaxCap(t *testing.T) {
	policy := &RetryPolicy{
		BaseDelay:    1 * time.Second,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	delay := policy.CalculateDelay(10) // Would be 512s without cap
	if delay > policy.MaxDelay {
		t.Errorf("delay %v exceeds max %v", delay, policy.MaxDelay)
	}
}

func TestRetryPolicy_CalculateDelay_Jitter(t *testing.T) {
	policy := &RetryPolicy{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 1.0, // Full jitter
	}

	// With full jitter, delays should vary
	delays := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		delay := policy.CalculateDelay(1)
		delays[delay] = true
	}

	if len(delays) < 10 {
		t.Error("jitter should produce varied delays")
	}
}

func TestDatabaseError_IsRetriable(t *testing.T) {
	tests := []struct {
		name      string
		err       *DatabaseError
		retriable bool
	}{
		{
			name:      "transient error",
			err:       &DatabaseError{Code: "Neo.TransientError.Network.Timeout"},
			retriable: true,
		},
		{
			name:      "memgraph conflict",
			err:       &DatabaseError{Message: "Cannot resolve conflicting transactions"},
			retriable: true,
		},
		{
			name:      "deadlock",
			err:       &DatabaseError{Code: "DeadlockDetected"},
			retriable: true,
		},
		{
			name:      "not a leader",
			err:       &DatabaseError{Message: "not a leader"},
			retriable: true,
		},
		{
			name:      "auth error",
			err:       &DatabaseError{Code: "Neo.ClientError.Security.Unauthorized"},
			retriable: false,
		},
		{
			name:      "syntax error",
			err:       &DatabaseError{Code: "Neo.ClientError.Statement.SyntaxError"},
			retriable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRetriable(); got != tt.retriable {
				t.Errorf("IsRetriable() = %v, want %v", got, tt.retriable)
			}
		})
	}
}

func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	policy := &RetryPolicy{MaxAttempts: 3}

	attempts := 0
	result, err := Retry(ctx, policy, func() (string, error) {
		attempts++
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	policy := &RetryPolicy{
		MaxAttempts:  5,
		BaseDelay:    1 * time.Millisecond,
		Multiplier:   1.0,
		JitterFactor: 0,
	}

	attempts := 0
	result, err := Retry(ctx, policy, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", &DatabaseError{Message: "timeout"}
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_ExhaustedRetries(t *testing.T) {
	ctx := context.Background()
	policy := &RetryPolicy{
		MaxAttempts:  3,
		BaseDelay:    1 * time.Millisecond,
		Multiplier:   1.0,
		JitterFactor: 0,
	}

	attempts := 0
	_, err := Retry(ctx, policy, func() (string, error) {
		attempts++
		return "", &DatabaseError{Message: "timeout"}
	})

	if err == nil {
		t.Fatal("expected error")
	}

	var retryErr *RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryError, got %T", err)
	}

	if retryErr.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", retryErr.Attempts)
	}
}

func TestRetry_NonRetriableError(t *testing.T) {
	ctx := context.Background()
	policy := &RetryPolicy{MaxAttempts: 5}

	attempts := 0
	_, err := Retry(ctx, policy, func() (string, error) {
		attempts++
		return "", &DatabaseError{Code: "Neo.ClientError.Security.Unauthorized"}
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retriable error, got %d", attempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	policy := &RetryPolicy{
		MaxAttempts:  10,
		BaseDelay:    50 * time.Millisecond,
		Multiplier:   1.0,
		JitterFactor: 0,
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, policy, func() (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "", &DatabaseError{Message: "timeout"}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetry_Callbacks(t *testing.T) {
	ctx := context.Background()

	var retryCalls []RetryContext
	var successAttempts int
	var failureErr error

	policy := &RetryPolicy{
		MaxAttempts:  3,
		BaseDelay:    1 * time.Millisecond,
		Multiplier:   1.0,
		JitterFactor: 0,
		OnRetry: func(ctx RetryContext) {
			retryCalls = append(retryCalls, ctx)
		},
		OnSuccess: func(attempts int) {
			successAttempts = attempts
		},
		OnFailure: func(err error, _ int) {
			failureErr = err
		},
	}

	attempts := 0
	_, _ = Retry(ctx, policy, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", &DatabaseError{Message: "timeout"}
		}
		return "ok", nil
	})

	if len(retryCalls) != 2 {
		t.Errorf("expected 2 retry callbacks, got %d", len(retryCalls))
	}
	if successAttempts != 3 {
		t.Errorf("expected success on attempt 3, got %d", successAttempts)
	}
	if failureErr != nil {
		t.Errorf("unexpected failure callback: %v", failureErr)
	}
}
