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

	fieldsW := messageIn.Fields()
	if len(fieldsW) != 1 {
		return nil, nil, errors.New("invalid fields length")
	}
	fieldsCols := fieldsW[0].(map[string]interface{})["fields"].([]interface{})

	strFieldsCols := []string{}
	for _, col := range fieldsCols {
		strFieldsCols = append(strFieldsCols, col.(string))
	}

	allData := []map[string]interface{}{}

	pull := NewPull(map[string]interface{}{
		"n":   -1,
		"qid": -1,
	})

	i := 0
	for {
		i++
		var pullResponse Message
		pullResponse, err = sendRequest(pull.Signature(), pull.Fields(), conn)
		if err != nil {
			break
		}

		colsValues, isArray := pullResponse.Fields()[0].([]interface{})
		if !isArray {
			return strFieldsCols, allData, nil
		}

		row := map[string]interface{}{}
		for i, field := range strFieldsCols {
			row[field] = colsValues[i]
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
		return nil, errors.New("Too many fields in message structure")
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
	defer conn.SetReadDeadline(time.Time{})

	for {
		sizeBytes := make([]byte, 2)
		if _, err := io.ReadFull(conn, sizeBytes); err != nil {
			if err == io.EOF {
				return nil, errors.New("Connection closed while reading chunk header")
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
			return nil, errors.New(fmt.Sprintf("Error reading chunk data: %v", err))
		}

		// Append to message buffer
		messageData.Write(chunk)
	}
	reader := packstream.NewUnpacker(&messageData)
	unpacked, err := reader.Unpack()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error unpacking chunk data: %v", err))
	}

	items := unpacked.([]interface{})
	signature, fields := items[0].(byte), items[1].([]interface{})

	msg, err := CreateMessage(signature, fields)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error creating message: %v", err))
	}
	return msg, nil
}
