package messaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/seuros/gopher-cypher/src/bolt/packstream"
)

// DefaultReadTimeout is the default timeout for reading from the connection
const DefaultReadTimeout = 30 * time.Second

func sendRequest(signature byte, fields []interface{}, conn net.Conn) (Message, error) {
	messageBytes, err := packMessage(signature, fields)
	if err != nil {
		return nil, err
	}
	messageSize := len(messageBytes)
	chunkHeader := make([]byte, 2)
	binary.BigEndian.PutUint16(chunkHeader, uint16(messageSize))
	_, err = conn.Write(chunkHeader)
	if err != nil {
		return nil, err
	}
	_, err = conn.Write(messageBytes)
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte{0x00, 0x00})
	if err != nil {
		return nil, err
	}
	messageIn, err := readChunkedMessage(conn)
	if err != nil {
		return nil, err
	}
	return messageIn, nil
}

func sendRequestData(signature byte, fields []interface{}, conn net.Conn) ([]string, []map[string]interface{}, error) {
	messageBytes, err := packMessage(signature, fields)
	if err != nil {
		return nil, nil, err
	}
	messageSize := len(messageBytes)
	chunkHeader := make([]byte, 2)
	binary.BigEndian.PutUint16(chunkHeader, uint16(messageSize))
	_, err = conn.Write(chunkHeader)
	if err != nil {
		return nil, nil, err
	}
	_, err = conn.Write(messageBytes)
	if err != nil {
		return nil, nil, err
	}
	_, err = conn.Write([]byte{0x00, 0x00})
	if err != nil {
		return nil, nil, err
	}

	messageIn, err := readChunkedMessage(conn)
	if err != nil {
		return nil, nil, err
	}

	// Check for FAILURE response first
	if messageIn.Signature() == FailureSignature {
		if failure, ok := messageIn.(*Failure); ok {
			return nil, nil, fmt.Errorf("query failed: [%s] %s", failure.Code(), failure.Message())
		}
		return nil, nil, errors.New("query execution failed")
	}

	// Check for unexpected response types
	if messageIn.Signature() != SuccessSignature {
		return nil, nil, fmt.Errorf("unexpected response type: 0x%02X", messageIn.Signature())
	}

	fieldsW := messageIn.Fields()
	if len(fieldsW) != 1 {
		return nil, nil, errors.New("invalid fields length")
	}

	// Safely extract fields with type checking
	fieldsMap, ok := fieldsW[0].(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("invalid response format: expected map")
	}

	fieldsVal, exists := fieldsMap["fields"]
	if !exists {
		return nil, nil, errors.New("invalid response format: missing 'fields' key")
	}

	fieldsCols, ok := fieldsVal.([]interface{})
	if !ok {
		// Handle nil or unexpected type
		if fieldsVal == nil {
			fieldsCols = []interface{}{}
		} else {
			return nil, nil, fmt.Errorf("invalid response format: 'fields' is %T, expected []interface{}", fieldsVal)
		}
	}

	strFieldsCols := make([]string, 0, len(fieldsCols))
	for _, col := range fieldsCols {
		if colStr, ok := col.(string); ok {
			strFieldsCols = append(strFieldsCols, colStr)
		} else if col == nil {
			strFieldsCols = append(strFieldsCols, "")
		} else {
			strFieldsCols = append(strFieldsCols, fmt.Sprintf("%v", col))
		}
	}

	allData := []map[string]interface{}{}

	pull := NewPull(map[string]interface{}{
		"n":   -1,
		"qid": -1,
	})

	for {
		var pullResponse Message
		pullResponse, err = sendRequest(pull.Signature(), pull.Fields(), conn)
		if err != nil {
			break
		}

		// Check for FAILURE in PULL response
		if pullResponse.Signature() == FailureSignature {
			if failure, ok := pullResponse.(*Failure); ok {
				return nil, nil, fmt.Errorf("pull failed: [%s] %s", failure.Code(), failure.Message())
			}
			return nil, nil, errors.New("pull failed")
		}

		// SUCCESS with no more records
		pullFields := pullResponse.Fields()
		if len(pullFields) == 0 {
			return strFieldsCols, allData, nil
		}

		// Check if this is a SUCCESS (end of results) vs RECORD
		if pullResponse.Signature() == SuccessSignature {
			return strFieldsCols, allData, nil
		}

		colsValues, isArray := pullFields[0].([]interface{})
		if !isArray {
			return strFieldsCols, allData, nil
		}

		row := make(map[string]interface{}, len(strFieldsCols))
		for i, field := range strFieldsCols {
			if i < len(colsValues) {
				row[field] = colsValues[i]
			} else {
				row[field] = nil
			}
		}
		allData = append(allData, row)
	}

	return strFieldsCols, allData, nil
}

// PackMessage exports the packMessage function for use by streaming connections
func PackMessage(signature byte, fields []interface{}) ([]byte, error) {
	return packMessage(signature, fields)
}

func packMessage(signature byte, fields []interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	writer := packstream.NewPacker(&buffer)
	structSize := len(fields)
	if structSize < 16 {
		buffer.WriteByte(0xB0 | byte(structSize))
	} else {
		return nil, errors.New("too many fields in message structure")
	}

	buffer.WriteByte(signature)
	for _, field := range fields {
		if err := writer.Pack(field); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

// ReadChunkedMessage exports the readChunkedMessage function for use by streaming connections
func ReadChunkedMessage(conn net.Conn) (Message, error) {
	return readChunkedMessage(conn)
}

func readChunkedMessage(conn net.Conn) (Message, error) {
	var messageData bytes.Buffer

	// Set read deadline to prevent hanging
	if err := conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	// Clear deadline when done
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	for {
		sizeBytes := make([]byte, 2)
		if _, err := io.ReadFull(conn, sizeBytes); err != nil {
			if err == io.EOF {
				return nil, errors.New("connection closed while reading chunk header")
			}
			return nil, fmt.Errorf("error reading chunk header: %w", err)
		}

		chunkSize := binary.BigEndian.Uint16(sizeBytes)

		// If chunk size is 0, we've reached the end of the message
		if chunkSize == 0 {
			break
		}

		// Read chunk data
		chunk := make([]byte, chunkSize)
		if _, err := io.ReadFull(conn, chunk); err != nil {
			return nil, fmt.Errorf("error reading chunk data: %w", err)
		}

		// Append to message buffer
		messageData.Write(chunk)
	}
	reader := packstream.NewUnpacker(&messageData)
	unpacked, err := reader.Unpack()
	if err != nil {
		return nil, fmt.Errorf("error unpacking chunk data: %w", err)
	}

	items, ok := unpacked.([]interface{})
	if !ok || len(items) < 2 {
		return nil, errors.New("invalid message structure: expected [signature, fields]")
	}

	signature, ok := items[0].(byte)
	if !ok {
		return nil, fmt.Errorf("invalid message signature type: %T", items[0])
	}

	fields, ok := items[1].([]interface{})
	if !ok {
		// Handle nil fields
		if items[1] == nil {
			fields = []interface{}{}
		} else {
			return nil, fmt.Errorf("invalid message fields type: %T", items[1])
		}
	}

	msg, err := CreateMessage(signature, fields)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	return msg, nil
}
