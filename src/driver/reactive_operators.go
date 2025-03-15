package driver

import (
	"context"
	"sync"
	"time"
)

// Transform operator implementation
func (r *reactiveResult) Transform(fn TransformFunc) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &transformOperator{fn: fn})
	return newResult
}

type transformOperator struct {
	fn TransformFunc
}

func (op *transformOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil && op.fn != nil {
				transformed := op.fn(event.Record)
				event.Record = transformed
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Filter operator implementation
func (r *reactiveResult) Filter(fn FilterFunc) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &filterOperator{fn: fn})
	return newResult
}

type filterOperator struct {
	fn FilterFunc
}

func (op *filterOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			// Pass through non-record events
			if event.Record == nil || op.fn == nil || op.fn(event.Record) {
				select {
				case output <- event:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Map operator implementation
func (r *reactiveResult) Map(fn MapFunc) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &mapOperator{fn: fn})
	return newResult
}

type mapOperator struct {
	fn MapFunc
}

func (op *mapOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil && op.fn != nil {
				mapped := op.fn(event.Record)
				// Convert mapped result back to Record format
				if mappedRecord, ok := mapped.(Record); ok {
					event.Record = &mappedRecord
				} else if mappedMap, ok := mapped.(map[string]interface{}); ok {
					mappedRecord := Record(mappedMap)
					event.Record = &mappedRecord
				}
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Batch operator implementation
func (r *reactiveResult) Batch(size int) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &batchOperator{size: size})
	return newResult
}

type batchOperator struct {
	size int
}

func (op *batchOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	batch := make([]*Record, 0, op.size)

	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil {
				batch = append(batch, event.Record)

				if len(batch) >= op.size {
					// Emit batch as a single record containing a slice
					batchRecord := Record{"batch": batch}
					select {
					case output <- RecordEvent{Record: &batchRecord}:
					case <-ctx.Done():
						return ctx.Err()
					}
					batch = batch[:0] // Clear batch but keep capacity
				}
			} else {
				// Handle completion or error events
				if len(batch) > 0 {
					// Emit remaining batch
					batchRecord := Record{"batch": batch}
					select {
					case output <- RecordEvent{Record: &batchRecord}:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				// Forward the completion/error event
				select {
				case output <- event:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// BatchByTime operator implementation
func (r *reactiveResult) BatchByTime(duration time.Duration) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &batchByTimeOperator{duration: duration})
	return newResult
}

type batchByTimeOperator struct {
	duration time.Duration
}

func (op *batchByTimeOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	batch := make([]*Record, 0, 100)
	timer := time.NewTimer(op.duration)
	defer timer.Stop()

	emitBatch := func() {
		if len(batch) > 0 {
			batchRecord := Record{"batch": batch}
			select {
			case output <- RecordEvent{Record: &batchRecord}:
			case <-ctx.Done():
			}
			batch = batch[:0]
		}
		timer.Reset(op.duration)
	}

	for {
		select {
		case event, ok := <-input:
			if !ok {
				emitBatch()
				return nil
			}

			if event.Record != nil {
				batch = append(batch, event.Record)
			} else {
				// Handle completion or error
				emitBatch()
				select {
				case output <- event:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

		case <-timer.C:
			emitBatch()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Take operator implementation
func (r *reactiveResult) Take(n int64) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &takeOperator{n: n})
	return newResult
}

type takeOperator struct {
	n int64
}

func (op *takeOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	count := int64(0)

	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil {
				if count >= op.n {
					// Emit completion event
					select {
					case output <- RecordEvent{Complete: true}:
					case <-ctx.Done():
					}
					return nil
				}
				count++
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Skip operator implementation
func (r *reactiveResult) Skip(n int64) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &skipOperator{n: n})
	return newResult
}

type skipOperator struct {
	n int64
}

func (op *skipOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	count := int64(0)

	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil {
				if count < op.n {
					count++
					continue // Skip this record
				}
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Distinct operator implementation
func (r *reactiveResult) Distinct(keyFunc func(*Record) string) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &distinctOperator{keyFunc: keyFunc})
	return newResult
}

type distinctOperator struct {
	keyFunc func(*Record) string
	seen    sync.Map
}

func (op *distinctOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil && op.keyFunc != nil {
				key := op.keyFunc(event.Record)
				if _, exists := op.seen.LoadOrStore(key, true); exists {
					continue // Skip duplicate
				}
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Throttle operator implementation
func (r *reactiveResult) Throttle(rate time.Duration) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &throttleOperator{rate: rate})
	return newResult
}

type throttleOperator struct {
	rate time.Duration
}

func (op *throttleOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	ticker := time.NewTicker(op.rate)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil {
				// Wait for next tick before emitting record
				select {
				case <-ticker.C:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Side effect operators
func (r *reactiveResult) OnError(handler ErrorHandler) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &onErrorOperator{handler: handler})
	return newResult
}

type onErrorOperator struct {
	handler ErrorHandler
}

func (op *onErrorOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Error != nil && op.handler != nil {
				recoveredErr := op.handler(event.Error)
				if recoveredErr != nil {
					event.Error = recoveredErr
				} else {
					// Error was handled, continue stream
					continue
				}
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *reactiveResult) DoOnNext(action func(*Record)) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &doOnNextOperator{action: action})
	return newResult
}

type doOnNextOperator struct {
	action func(*Record)
}

func (op *doOnNextOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Record != nil && op.action != nil {
				op.action(event.Record)
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *reactiveResult) DoOnComplete(action func(*ResultSummary)) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &doOnCompleteOperator{action: action})
	return newResult
}

type doOnCompleteOperator struct {
	action func(*ResultSummary)
}

func (op *doOnCompleteOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Complete && event.Summary != nil && op.action != nil {
				op.action(event.Summary)
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *reactiveResult) DoOnError(action func(error)) ReactiveResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	newResult := r.copy()
	newResult.operators = append(newResult.operators, &doOnErrorOperator{action: action})
	return newResult
}

type doOnErrorOperator struct {
	action func(error)
}

func (op *doOnErrorOperator) apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error {
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return nil
			}
			if event.Error != nil && op.action != nil {
				op.action(event.Error)
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Helper method to copy reactive result for operator chaining
func (r *reactiveResult) copy() *reactiveResult {
	operators := make([]reactiveOperator, len(r.operators))
	copy(operators, r.operators)

	return &reactiveResult{
		source:      r.source,
		query:       r.query,
		params:      r.params,
		config:      r.config,
		operators:   operators,
		logger:      r.logger,
		observables: r.observables,
	}
}
