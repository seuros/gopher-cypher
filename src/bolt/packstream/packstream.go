package packstream

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
)

// Marker Bytes & Limits
const (
	TINY_STRING_MARKER_BASE = 0x80
	STRING_8_MARKER         = 0xD0
	STRING_16_MARKER        = 0xD1
	// STRING_32_MARKER        = 0xD2 // Not implementing 32-bit sizes for now

	TINY_LIST_MARKER_BASE = 0x90
	LIST_8_MARKER         = 0xD4
	LIST_16_MARKER        = 0xD5
	// LIST_32_MARKER         = 0xD6 // Not implementing 32-bit sizes for now

	TINY_MAP_MARKER_BASE = 0xA0
	MAP_8_MARKER         = 0xD8
	MAP_16_MARKER        = 0xD9
	// MAP_32_MARKER        = 0xDA // Not implementing 32-bit sizes for now

	INT_8  = 0xC8
	INT_16 = 0xC9
	INT_32 = 0xCA
	INT_64 = 0xCB

	NULL    = 0xC0
	FALSEY  = 0xC2
	TRUETHY = 0xC3

	FLOAT_64 = 0xC1

	TINY_STRUCT_MARKER_BASE = 0xB0
	STRUCT_8_MARKER         = 0xDC
	STRUCT_16_MARKER        = 0xDD
	STRUCT_32_MARKER        = 0xDE

	TINY_INT_MIN = -16
	TINY_INT_MAX = 127
	INT_8_MIN    = -128
	INT_8_MAX    = 127
	INT_16_MIN   = -32768
	INT_16_MAX   = 32767
	INT_32_MIN   = -2147483648
	INT_32_MAX   = 2147483647
	// INT_64 limits are typically Go's standard int64 limits

	MARKER_HIGH_NIBBLE_MASK = 0xF0
	MARKER_LOW_NIBBLE_MASK  = 0x0F
)

// ProtocolError is raised for protocol violations
type ProtocolError struct {
	Message string
}

func (e ProtocolError) Error() string {
	return e.Message
}

// Packer handles serializing Go types to Packstream format
type Packer struct {
	writer io.Writer
}

// NewPacker creates a new Packstream packer
func NewPacker(writer io.Writer) *Packer {
	return &Packer{writer: writer}
}

// Pack serializes a value to Packstream format
func (p *Packer) Pack(value interface{}) error {
	switch v := value.(type) {
	case string:
		return p.packString(v)
	case map[string]interface{}:
		return p.packMap(v)
	case []byte:
		// PackStream supports a dedicated "Bytes" type, but this implementation
		// does not currently implement it. Without this case, the reflect-slice
		// fallback would start writing a list header and then fail on uint8 elems.
		return &ProtocolError{Message: "Cannot pack type: []byte (bytes are not supported)"}
	case int, int8, int16, int32, int64:
		// Convert to int64 for consistent handling
		var intValue int64
		switch iv := v.(type) {
		case int:
			intValue = int64(iv)
		case int8:
			intValue = int64(iv)
		case int16:
			intValue = int64(iv)
		case int32:
			intValue = int64(iv)
		case int64:
			intValue = iv
		}
		return p.packInteger(intValue)
	case bool:
		if v {
			return p.writeMarker([]byte{TRUETHY})
		}
		return p.writeMarker([]byte{FALSEY})
	case nil:
		return p.writeMarker([]byte{NULL})
	case []interface{}:
		return p.packList(v)
	default:
		// Use reflection to handle typed slices ([]string, []int, etc.)
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice {
			return p.packReflectSlice(rv)
		}
		return &ProtocolError{Message: fmt.Sprintf("Cannot pack type: %T", v)}
	}
}

func (p *Packer) packListHeader(size int) error {
	if size < 16 { // TinyList
		return p.writeMarker([]byte{TINY_LIST_MARKER_BASE | byte(size)})
	}
	if size < 256 { // LIST_8
		return p.writeMarker([]byte{LIST_8_MARKER, byte(size)})
	}
	if size < 65536 { // LIST_16
		var header [3]byte
		header[0] = LIST_16_MARKER
		binary.BigEndian.PutUint16(header[1:], uint16(size))
		return p.writeMarker(header[:])
	}

	return &ProtocolError{Message: fmt.Sprintf("List too large to pack (size: %d)", size)}
}

func (p *Packer) packString(str string) error {
	bytes := []byte(str)
	size := len(bytes)

	if size < 16 { // TinyString
		return p.writeMarkerAndData([]byte{TINY_STRING_MARKER_BASE | byte(size)}, bytes)
	} else if size < 256 { // STRING_8
		return p.writeMarkerAndData([]byte{STRING_8_MARKER, byte(size)}, bytes)
	} else if size < 65536 { // STRING_16
		marker := []byte{STRING_16_MARKER}
		sizeBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(sizeBytes, uint16(size))
		return p.writeMarkerAndData(append(marker, sizeBytes...), bytes)
	} else {
		return &ProtocolError{Message: fmt.Sprintf("String too large to pack (size: %d)", size)}
	}
}

func (p *Packer) packMap(m map[string]interface{}) error {
	size := len(m)

	if size < 16 { // TinyMap
		if err := p.writeMarker([]byte{TINY_MAP_MARKER_BASE | byte(size)}); err != nil {
			return err
		}
	} else if size < 256 { // MAP_8
		if err := p.writeMarker([]byte{MAP_8_MARKER, byte(size)}); err != nil {
			return err
		}
	} else if size < 65536 { // MAP_16
		marker := []byte{MAP_16_MARKER}
		sizeBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(sizeBytes, uint16(size))
		if err := p.writeMarker(append(marker, sizeBytes...)); err != nil {
			return err
		}
	} else {
		return &ProtocolError{Message: fmt.Sprintf("Map too large to pack (size: %d)", size)}
	}

	// Write key-value pairs
	for key, value := range m {
		if err := p.Pack(key); err != nil {
			return err
		}
		if err := p.Pack(value); err != nil {
			return err
		}
	}

	return nil
}

func (p *Packer) packList(list []interface{}) error {
	if err := p.packListHeader(len(list)); err != nil {
		return err
	}

	// Pack each item in the list
	for _, item := range list {
		if err := p.Pack(item); err != nil {
			return err
		}
	}

	return nil
}

// packReflectSlice handles typed slices via reflection ([]string, []int, etc.)
func (p *Packer) packReflectSlice(rv reflect.Value) error {
	size := rv.Len()
	if err := p.packListHeader(size); err != nil {
		return err
	}

	// Pack each item
	for i := 0; i < size; i++ {
		if err := p.Pack(rv.Index(i).Interface()); err != nil {
			return err
		}
	}

	return nil
}

func (p *Packer) packInteger(i int64) error {
	if i >= TINY_INT_MIN && i <= TINY_INT_MAX { // Tiny Integer
		return p.writeMarker([]byte{byte(i)})
	} else if i >= INT_8_MIN && i <= INT_8_MAX { // INT_8
		return p.writeMarkerAndData([]byte{INT_8}, []byte{byte(i)})
	} else if i >= INT_16_MIN && i <= INT_16_MAX { // INT_16
		marker := []byte{INT_16}
		valueBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(valueBytes, uint16(i))
		return p.writeMarkerAndData(marker, valueBytes)
	} else if i >= INT_32_MIN && i <= INT_32_MAX { // INT_32
		marker := []byte{INT_32}
		valueBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(valueBytes, uint32(i))
		return p.writeMarkerAndData(marker, valueBytes)
	} else { // INT_64
		marker := []byte{INT_64}
		valueBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(valueBytes, uint64(i))
		return p.writeMarkerAndData(marker, valueBytes)
	}
}

func (p *Packer) writeMarker(markerBytes []byte) error {
	_, err := p.writer.Write(markerBytes)
	return err
}

func (p *Packer) writeMarkerAndData(markerBytes, dataBytes []byte) error {
	if _, err := p.writer.Write(markerBytes); err != nil {
		return err
	}
	if dataBytes != nil && len(dataBytes) > 0 {
		_, err := p.writer.Write(dataBytes)
		return err
	}
	return nil
}

// Unpacker handles deserializing Packstream format to Go types
type Unpacker struct {
	reader io.Reader
}

// NewUnpacker creates a new Packstream unpacker
func NewUnpacker(reader io.Reader) *Unpacker {
	return &Unpacker{reader: reader}
}

// Unpack deserializes the next value from the stream
func (u *Unpacker) Unpack() (interface{}, error) {
	marker, err := u.readByte()
	if err != nil {
		return nil, err
	}
	return u.unpackValue(marker)
}

func (u *Unpacker) unpackValue(marker byte) (interface{}, error) {
	// Tiny types
	if marker >= 0 && marker < TINY_STRING_MARKER_BASE { // Tiny Positive Int
		return int64(marker), nil
	}
	if marker >= 0xF0 { // Tiny Negative Int (-1 to -16)
		return int64(int8(marker)), nil
	}

	highNibble := marker & MARKER_HIGH_NIBBLE_MASK
	lowNibble := marker & MARKER_LOW_NIBBLE_MASK

	switch highNibble {
	case TINY_STRING_MARKER_BASE:
		return u.unpackString(int(lowNibble))
	case TINY_LIST_MARKER_BASE:
		return u.unpackList(int(lowNibble))
	case TINY_MAP_MARKER_BASE:
		return u.unpackMap(int(lowNibble))
	case TINY_STRUCT_MARKER_BASE:
		return u.unpackStructure(int(lowNibble))
	}

	// Other markers
	switch marker {
	case NULL:
		return nil, nil
	case FALSEY:
		return false, nil
	case TRUETHY:
		return true, nil
	case INT_8:
		return u.readInt(1)
	case INT_16:
		return u.readInt(2)
	case INT_32:
		return u.readInt(4)
	case INT_64:
		return u.readInt(8)
	case STRING_8_MARKER:
		size, err := u.readSize(1)
		if err != nil {
			return nil, err
		}
		return u.unpackString(int(size))
	case STRING_16_MARKER:
		size, err := u.readSize(2)
		if err != nil {
			return nil, err
		}
		return u.unpackString(int(size))
	case LIST_8_MARKER:
		size, err := u.readSize(1)
		if err != nil {
			return nil, err
		}
		return u.unpackList(int(size))
	case LIST_16_MARKER:
		size, err := u.readSize(2)
		if err != nil {
			return nil, err
		}
		return u.unpackList(int(size))
	case MAP_8_MARKER:
		size, err := u.readSize(1)
		if err != nil {
			return nil, err
		}
		return u.unpackMap(int(size))
	case MAP_16_MARKER:
		size, err := u.readSize(2)
		if err != nil {
			return nil, err
		}
		return u.unpackMap(int(size))
	case STRUCT_8_MARKER:
		size, err := u.readSize(1)
		if err != nil {
			return nil, err
		}
		return u.unpackStructure(int(size))
	case STRUCT_16_MARKER:
		size, err := u.readSize(2)
		if err != nil {
			return nil, err
		}
		return u.unpackStructure(int(size))
	case FLOAT_64:
		return u.unpackFloat64()
	default:
		return nil, &ProtocolError{Message: fmt.Sprintf("Unknown Packstream marker: 0x%x", marker)}
	}
}

func (u *Unpacker) unpackString(size int) (string, error) {
	bytes, err := u.readBytes(size)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (u *Unpacker) unpackList(size int) ([]interface{}, error) {
	result := make([]interface{}, size)
	for i := 0; i < size; i++ {
		value, err := u.Unpack()
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

func (u *Unpacker) unpackMap(size int) (map[string]interface{}, error) {
	result := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		keyVal, err := u.Unpack()
		if err != nil {
			return nil, err
		}

		key, ok := keyVal.(string)
		if !ok {
			return nil, &ProtocolError{Message: "Map key must be a string"}
		}

		value, err := u.Unpack()
		if err != nil {
			return nil, err
		}

		result[key] = value
	}
	return result, nil
}

// Unpacks a structure into a [signature, [fields]] array
func (u *Unpacker) unpackStructure(size int) ([]interface{}, error) {
	signature, err := u.readByte()
	if err != nil {
		return nil, err
	}

	fields := make([]interface{}, size)
	for i := 0; i < size; i++ {
		field, err := u.Unpack()
		if err != nil {
			return nil, err
		}
		fields[i] = field
	}

	return []interface{}{signature, fields}, nil
}

func (u *Unpacker) unpackFloat64() (float64, error) {
	bytes, err := u.readBytes(8)
	if err != nil {
		return 0, err
	}
	bits := binary.BigEndian.Uint64(bytes)
	return math.Float64frombits(bits), nil
}

func (u *Unpacker) readByte() (byte, error) {
	bytes := make([]byte, 1)
	_, err := io.ReadFull(u.reader, bytes)
	if err != nil {
		if err == io.EOF {
			return 0, &ProtocolError{Message: "Unexpected end of stream while reading byte"}
		}
		return 0, err
	}
	return bytes[0], nil
}

func (u *Unpacker) readSize(numBytes int) (uint64, error) {
	bytes, err := u.readBytes(numBytes)
	if err != nil {
		return 0, err
	}

	switch numBytes {
	case 1:
		return uint64(bytes[0]), nil
	case 2:
		return uint64(binary.BigEndian.Uint16(bytes)), nil
	case 4:
		return uint64(binary.BigEndian.Uint32(bytes)), nil
	default:
		return 0, &ProtocolError{Message: fmt.Sprintf("Invalid size length: %d", numBytes)}
	}
}

func (u *Unpacker) readInt(numBytes int) (int64, error) {
	bytes, err := u.readBytes(numBytes)
	if err != nil {
		return 0, err
	}

	switch numBytes {
	case 1:
		return int64(int8(bytes[0])), nil
	case 2:
		return int64(int16(binary.BigEndian.Uint16(bytes))), nil
	case 4:
		return int64(int32(binary.BigEndian.Uint32(bytes))), nil
	case 8:
		return int64(binary.BigEndian.Uint64(bytes)), nil
	default:
		return 0, &ProtocolError{Message: fmt.Sprintf("Invalid int size: %d", numBytes)}
	}
}

func (u *Unpacker) readBytes(n int) ([]byte, error) {
	if n == 0 {
		return []byte{}, nil
	}

	data := make([]byte, n)
	bytesRead, err := io.ReadFull(u.reader, data)
	if err != nil {
		return nil, &ProtocolError{Message: fmt.Sprintf("Unexpected end of stream while reading %d bytes (got %d)", n, bytesRead)}
	}

	return data, nil
}

// Pack is a helper function to pack a value into a byte slice
func Pack(value interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	packer := NewPacker(buffer)
	err := packer.Pack(value)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Unpack is a helper function to unpack a value from a byte slice
func Unpack(data []byte) (interface{}, error) {
	buffer := bytes.NewBuffer(data)
	unpacker := NewUnpacker(buffer)
	return unpacker.Unpack()
}
