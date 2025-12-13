package driver

import (
	"context"
	"time"

	"github.com/seuros/gopher-cypher/src/bolt/messaging"
	"github.com/yudhasubki/netpool"
)

// streamingConnectionWrapper implements StreamConnection interface
type streamingConnectionWrapper struct {
	conn          *pooledConn
	netPool       *netpool.Netpool
	query         string
	params        map[string]interface{}
	metaData      map[string]interface{}
	keys          []string
	hasKeys       bool
	exhausted     bool
	closed        bool
	logger        Logger
	config        *Config
	observability *observabilityInstruments
	spanCtx       *spanContext
	summary       *ResultSummary
	startTime     time.Time
	lastErr       error
	pending       []*Record
}

func (sc *streamingConnectionWrapper) sendRun(ctx context.Context) error {
	if sc.config.Logging != nil && sc.config.Logging.LogBoltMessages {
		sc.logger.Debug("Sending RUN message for streaming", "query_type", sc.summary.QueryType)
	}

	// Send RUN message
	runMessage := messaging.NewRun(sc.query, sc.params, sc.metaData)

	// Pack and send RUN message
	messageBytes, err := messaging.PackMessage(runMessage.Signature(), runMessage.Fields())
	if err != nil {
		sc.lastErr = err
		return err
	}

	err = sc.writeChunkedMessage(messageBytes)
	if err != nil {
		sc.lastErr = err
		return err
	}

	// Read SUCCESS response with field metadata
	response, err := messaging.ReadChunkedMessage(sc.conn.Conn)
	if err != nil {
		sc.lastErr = err
		return err
	}

	if response.Signature() == messaging.FailureSignature {
		if failure, ok := response.(*messaging.Failure); ok {
			dbErr := &DatabaseError{
				Code:    failure.Code(),
				Message: failure.Message(),
			}
			sc.lastErr = dbErr
			return dbErr
		}
		usageErr := NewUsageError("Query execution failed")
		sc.lastErr = usageErr
		return usageErr
	}

	if response.Signature() != messaging.SuccessSignature {
		usageErr := NewUsageError("Unexpected response to RUN message")
		sc.lastErr = usageErr
		return usageErr
	}

	// Extract keys from SUCCESS response
	fields := response.Fields()
	if len(fields) > 0 {
		if metadata, ok := fields[0].(map[string]interface{}); ok {
			if fieldsArray, exists := metadata["fields"]; exists {
				if fieldsList, ok := fieldsArray.([]interface{}); ok {
					sc.keys = make([]string, len(fieldsList))
					for i, field := range fieldsList {
						if fieldStr, ok := field.(string); ok {
							sc.keys[i] = fieldStr
						} else {
							// Log type mismatch and use empty string to avoid panic
							if sc.logger != nil {
								sc.logger.Warn("Field name is not a string", "index", i, "type", field)
							}
							sc.keys[i] = ""
						}
					}
					sc.hasKeys = true
				}
			}
		}
	}

	if !sc.hasKeys {
		usageErr := NewUsageError("Failed to extract field names from RUN response")
		sc.lastErr = usageErr
		return usageErr
	}

	return nil
}

func (sc *streamingConnectionWrapper) GetKeys() ([]string, error) {
	if !sc.hasKeys {
		return nil, NewUsageError("Keys not available")
	}
	return sc.keys, nil
}

func (sc *streamingConnectionWrapper) PullNext(ctx context.Context, batchSize int) (*Record, *ResultSummary, error) {
	if sc.exhausted || sc.closed {
		return nil, nil, nil
	}

	// Serve buffered records first (from a previous PULL response).
	if len(sc.pending) > 0 {
		record := sc.pending[0]
		sc.pending = sc.pending[1:]
		return record, nil, nil
	}

	if batchSize <= 0 {
		batchSize = 1
	}

	// Touch connection to update last used time
	sc.conn.touch()

	// Send PULL message
	pullMsg := messaging.NewPull(map[string]interface{}{
		"n":   batchSize,
		"qid": -1,
	})

	messageBytes, err := messaging.PackMessage(pullMsg.Signature(), pullMsg.Fields())
	if err != nil {
		sc.lastErr = err
		return nil, nil, err
	}

	err = sc.writeChunkedMessage(messageBytes)
	if err != nil {
		sc.lastErr = err
		return nil, nil, err
	}

	// A single PULL can yield multiple RECORD messages followed by a terminating
	// SUCCESS/FAILURE. Read until the terminal message to keep the connection in
	// a consistent state for subsequent PULLs.
	for {
		response, err := messaging.ReadChunkedMessage(sc.conn.Conn)
		if err != nil {
			sc.lastErr = err
			return nil, nil, err
		}

		switch response.Signature() {
		case messaging.RecordSignature:
			fields := response.Fields()
			if len(fields) != 1 {
				usageErr := NewUsageError("Invalid RECORD format")
				sc.lastErr = usageErr
				return nil, nil, usageErr
			}
			values, ok := fields[0].([]interface{})
			if !ok {
				usageErr := NewUsageError("Invalid RECORD format")
				sc.lastErr = usageErr
				return nil, nil, usageErr
			}
			record := make(Record)
			for i, key := range sc.keys {
				if i < len(values) {
					record[key] = values[i]
				}
			}
			sc.pending = append(sc.pending, &record)

		case messaging.SuccessSignature:
			// Determine whether more records remain (Bolt "has_more" metadata).
			hasMore := false
			fields := response.Fields()
			if len(fields) > 0 {
				if metadata, ok := fields[0].(map[string]interface{}); ok {
					if v, ok := metadata["has_more"].(bool); ok {
						hasMore = v
					}

					// Only the final SUCCESS (has_more == false) is treated as end-of-stream.
					if !hasMore {
						// Update summary with final statistics
						if stats, exists := metadata["stats"]; exists {
							sc.summary.updateFromStats(stats)
						}
						if bookmark, exists := metadata["bookmark"]; exists {
							if bookmarkStr, ok := bookmark.(string); ok {
								sc.summary.Bookmark = bookmarkStr
							} else if sc.logger != nil {
								sc.logger.Warn("Bookmark is not a string", "type", bookmark)
							}
						}
					}
				}
			}

			if !hasMore {
				sc.exhausted = true

				sc.summary.ExecutionTime = time.Since(sc.startTime)

				// Log completion
				if sc.config.Logging != nil && sc.config.Logging.LogQueryTiming {
					sc.logger.Info("Streaming query completed", "duration", sc.summary.ExecutionTime, "query_type", sc.summary.QueryType)
				}

				// Finish observability span
				if sc.observability != nil && sc.config.Observability != nil {
					sc.observability.finishQuerySpan(sc.spanCtx, sc.summary, nil, sc.config.Observability)
				}
			}

			// Return the first buffered record if we have one.
			if len(sc.pending) > 0 {
				record := sc.pending[0]
				sc.pending = sc.pending[1:]
				return record, nil, nil
			}
			if sc.exhausted {
				return nil, sc.summary, nil
			}
			return nil, nil, nil

		case messaging.FailureSignature:
			sc.exhausted = true
			if failure, ok := response.(*messaging.Failure); ok {
				dbErr := &DatabaseError{
					Code:    failure.Code(),
					Message: failure.Message(),
				}
				sc.lastErr = dbErr

				// Finish observability span with error
				if sc.observability != nil && sc.config.Observability != nil {
					sc.observability.finishQuerySpan(sc.spanCtx, sc.summary, dbErr, sc.config.Observability)
				}

				return nil, nil, dbErr
			}
			usageErr := NewUsageError("Query execution failed")
			sc.lastErr = usageErr
			return nil, nil, usageErr

		default:
			usageErr := NewUsageError("Unexpected response from server")
			sc.lastErr = usageErr
			return nil, nil, usageErr
		}
	}
}

func (sc *streamingConnectionWrapper) writeChunkedMessage(messageBytes []byte) error {
	messageSize := len(messageBytes)
	chunkHeader := make([]byte, 2)
	chunkHeader[0] = byte(messageSize >> 8)
	chunkHeader[1] = byte(messageSize & 0xFF)

	_, err := sc.conn.Write(chunkHeader)
	if err != nil {
		return err
	}

	_, err = sc.conn.Write(messageBytes)
	if err != nil {
		return err
	}

	// End chunk marker
	_, err = sc.conn.Write([]byte{0x00, 0x00})
	return err
}

func (sc *streamingConnectionWrapper) Close() error {
	if sc.closed {
		return nil
	}

	wasExhausted := sc.exhausted
	sc.closed = true
	sc.exhausted = true

	// Return connection to pool (pooledConn satisfies net.Conn)
	putErr := sc.lastErr
	if putErr == nil && !wasExhausted {
		// If the stream is closed before being fully consumed, it's safer to discard the
		// underlying connection to avoid reusing it in an unknown protocol state.
		putErr = NewUsageError("Stream closed before being fully consumed")
	}
	sc.netPool.Put(sc.conn, putErr)

	return nil
}

// updateFromStats updates result summary from query statistics
func (rs *ResultSummary) updateFromStats(stats interface{}) {
	if statsMap, ok := stats.(map[string]interface{}); ok {
		if nodesCreated, exists := statsMap["nodes-created"]; exists {
			if count, ok := nodesCreated.(int64); ok {
				rs.NodesCreated = count
			}
		}
		if nodesDeleted, exists := statsMap["nodes-deleted"]; exists {
			if count, ok := nodesDeleted.(int64); ok {
				rs.NodesDeleted = count
			}
		}
		if relationshipsCreated, exists := statsMap["relationships-created"]; exists {
			if count, ok := relationshipsCreated.(int64); ok {
				rs.RelationshipsCreated = count
			}
		}
		if relationshipsDeleted, exists := statsMap["relationships-deleted"]; exists {
			if count, ok := relationshipsDeleted.(int64); ok {
				rs.RelationshipsDeleted = count
			}
		}
		if propertiesSet, exists := statsMap["properties-set"]; exists {
			if count, ok := propertiesSet.(int64); ok {
				rs.PropertiesSet = count
			}
		}
		if labelsAdded, exists := statsMap["labels-added"]; exists {
			if count, ok := labelsAdded.(int64); ok {
				rs.LabelsAdded = count
			}
		}
		if labelsRemoved, exists := statsMap["labels-removed"]; exists {
			if count, ok := labelsRemoved.(int64); ok {
				rs.LabelsRemoved = count
			}
		}
		if indexesAdded, exists := statsMap["indexes-added"]; exists {
			if count, ok := indexesAdded.(int64); ok {
				rs.IndexesAdded = count
			}
		}
		if indexesRemoved, exists := statsMap["indexes-removed"]; exists {
			if count, ok := indexesRemoved.(int64); ok {
				rs.IndexesRemoved = count
			}
		}
		if constraintsAdded, exists := statsMap["constraints-added"]; exists {
			if count, ok := constraintsAdded.(int64); ok {
				rs.ConstraintsAdded = count
			}
		}
		if constraintsRemoved, exists := statsMap["constraints-removed"]; exists {
			if count, ok := constraintsRemoved.(int64); ok {
				rs.ConstraintsRemoved = count
			}
		}
		if containsUpdates, exists := statsMap["contains-updates"]; exists {
			if contains, ok := containsUpdates.(bool); ok {
				rs.ContainsUpdates = contains
			}
		}
		if containsSystemUpdates, exists := statsMap["contains-system-updates"]; exists {
			if contains, ok := containsSystemUpdates.(bool); ok {
				rs.ContainsSystemUpdates = contains
			}
		}
	}
}
