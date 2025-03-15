package packstream

import (
	"bytes"
	"reflect"
	"testing"
)

func TestPackInteger(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected []byte
	}{
		{"Tiny Int 0", 0, []byte{0x00}},
		{"Tiny Int Positive", 42, []byte{0x2A}},
		{"Tiny Int Negative", -15, []byte{0xF1}},
		{"Int8 Positive", 127, []byte{0x7F}},
		{"Int8 Negative", -128, []byte{0xC8, 0x80}},
		{"Int16 Positive", 32000, []byte{0xC9, 0x7D, 0x00}},
		{"Int16 Negative", -32000, []byte{0xC9, 0x83, 0x00}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := NewPacker(buf)
			err := p.Pack(test.input)
			if err != nil {
				t.Fatalf("Failed to pack: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, test.expected) {
				t.Errorf("Expected %X, got %X", test.expected, got)
			}

			// Test unpacking
			u := NewUnpacker(bytes.NewReader(got))
			val, err := u.Unpack()
			if err != nil {
				t.Fatalf("Failed to unpack: %v", err)
			}

			if val.(int64) != test.input {
				t.Errorf("Unpack returned %v, expected %v", val, test.input)
			}
		})
	}
}

func TestPackString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{"Empty String", "", []byte{0x80}},
		{"Small String", "hello", []byte{0x85, 0x68, 0x65, 0x6C, 0x6C, 0x6F}},
		{"String8", string(bytes.Repeat([]byte("a"), 20)), append([]byte{0xD0, 0x14}, bytes.Repeat([]byte("a"), 20)...)},
		{"String16", string(bytes.Repeat([]byte("a"), 300)), append([]byte{0xD1, 0x01, 0x2C}, bytes.Repeat([]byte("a"), 300)...)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := NewPacker(buf)
			err := p.Pack(test.input)
			if err != nil {
				t.Fatalf("Failed to pack: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, test.expected) {
				t.Errorf("Expected %X, got %X", test.expected, got)
			}

			// Test unpacking
			u := NewUnpacker(bytes.NewReader(got))
			val, err := u.Unpack()
			if err != nil {
				t.Fatalf("Failed to unpack: %v", err)
			}

			if val.(string) != test.input {
				t.Errorf("Unpack returned %v, expected %v", val, test.input)
			}
		})
	}
}

func TestPackList(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []byte
	}{
		{"Empty List", []interface{}{}, []byte{0x90}},
		{"Small List", []interface{}{1, 2, 3}, []byte{0x93, 0x01, 0x02, 0x03}},
		{"Mixed List", []interface{}{1, "hello", true}, []byte{0x93, 0x01, 0x85, 0x68, 0x65, 0x6C, 0x6C, 0x6F, 0xC3}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := NewPacker(buf)
			err := p.Pack(test.input)
			if err != nil {
				t.Fatalf("Failed to pack: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, test.expected) {
				t.Errorf("Expected %X, got %X", test.expected, got)
			}

			// Test unpacking
			u := NewUnpacker(bytes.NewReader(got))
			_, err = u.Unpack()
			if err != nil {
				t.Fatalf("Failed to unpack: %v", err)
			}

		})
	}
}

func TestPackMap(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{"Empty Map", map[string]interface{}{}},
		{"Simple Map", map[string]interface{}{"a": 1, "b": 2}},
		{"Nested Map", map[string]interface{}{
			"a":      1,
			"nested": map[string]interface{}{"x": true, "y": "hello"},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := NewPacker(buf)
			err := p.Pack(test.input)
			if err != nil {
				t.Fatalf("Failed to pack: %v", err)
			}

			// Test unpacking
			u := NewUnpacker(bytes.NewReader(buf.Bytes()))
			_, err = u.Unpack()
			if err != nil {
				t.Fatalf("Failed to unpack: %v", err)
			}

		})
	}
}

func TestPackBooleanAndNil(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []byte
	}{
		{"Nil Value", nil, []byte{0xC0}},
		{"True Value", true, []byte{0xC3}},
		{"False Value", false, []byte{0xC2}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := NewPacker(buf)
			err := p.Pack(test.input)
			if err != nil {
				t.Fatalf("Failed to pack: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, test.expected) {
				t.Errorf("Expected %X, got %X", test.expected, got)
			}

			// Test unpacking
			u := NewUnpacker(bytes.NewReader(got))
			val, err := u.Unpack()
			if err != nil {
				t.Fatalf("Failed to unpack: %v", err)
			}

			if !reflect.DeepEqual(val, test.input) {
				t.Errorf("Unpack returned %v, expected %v", val, test.input)
			}
		})
	}
}

func TestComplexObjects(t *testing.T) {
	// Complex nested objects
	obj := map[string]interface{}{
		"name":    "John",
		"age":     42,
		"active":  true,
		"hobbies": []interface{}{"reading", "coding", nil},
		"address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "Anytown",
			"zip":    12345,
		},
		"nullable": nil,
	}

	buf := &bytes.Buffer{}
	p := NewPacker(buf)
	err := p.Pack(obj)
	if err != nil {
		t.Fatalf("Failed to pack complex object: %v", err)
	}

	// Try unpacking
	u := NewUnpacker(bytes.NewReader(buf.Bytes()))
	val, err := u.Unpack()
	if err != nil {
		t.Fatalf("Failed to unpack complex object: %v", err)
	}

	// Convert returned map to the expected type
	resultMap, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", val)
	}

	// Check some values
	if resultMap["name"] != "John" {
		t.Errorf("Expected name 'John', got %v", resultMap["name"])
	}
	if resultMap["age"] != int64(42) { // Integers get unpacked as int64
		t.Errorf("Expected age 42, got %v", resultMap["age"])
	}
	if resultMap["active"] != true {
		t.Errorf("Expected active true, got %v", resultMap["active"])
	}
	if resultMap["nullable"] != nil {
		t.Errorf("Expected nullable nil, got %v", resultMap["nullable"])
	}

	// Check hobbies array
	hobbies, ok := resultMap["hobbies"].([]interface{})
	if !ok || len(hobbies) != 3 {
		t.Errorf("Expected hobbies array of length 3, got %v", resultMap["hobbies"])
	} else {
		if hobbies[0] != "reading" || hobbies[1] != "coding" || hobbies[2] != nil {
			t.Errorf("Hobbies array contents incorrect: %v", hobbies)
		}
	}

	// Check nested address map
	address, ok := resultMap["address"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected address map, got %T", resultMap["address"])
	} else {
		if address["street"] != "123 Main St" || address["city"] != "Anytown" {
			t.Errorf("Address contents incorrect: %v", address)
		}
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test Pack and Unpack helper functions
	value := map[string]interface{}{
		"id":   123,
		"name": "Test Item",
	}

	// Pack
	data, err := Pack(value)
	if err != nil {
		t.Fatalf("Pack helper failed: %v", err)
	}

	// Unpack
	result, err := Unpack(data)
	if err != nil {
		t.Fatalf("Unpack helper failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Got wrong type back: %T", result)
	}

	if resultMap["id"] != int64(123) || resultMap["name"] != "Test Item" {
		t.Errorf("Unpacked data doesn't match original: %v", resultMap)
	}
}

func TestPackingErrors(t *testing.T) {
	// Test packing unsupported type
	buf := &bytes.Buffer{}
	p := NewPacker(buf)
	err := p.Pack(complex(1, 2)) // Complex number isn't supported
	if err == nil {
		t.Errorf("Expected error for unsupported type, got none")
	}

	// Test  too large string
	hugeString := string(bytes.Repeat([]byte("a"), 70000))
	err = p.Pack(hugeString)
	if err == nil {
		t.Errorf("Expected error for huge string, got none")
	}

}

func TestCorruptedInputStream(t *testing.T) {
	// Test with corrupted/invalid input
	invalidInput := []byte{0xD0, 0x10} // String marker with length 16, but no data
	_, err := Unpack(invalidInput)
	if err == nil {
		t.Errorf("Expected error for corrupted input, got none")
	}

	// Test with invalid marker
	invalidMarker := []byte{0xE0} // Not a valid marker in the current implementation
	_, err = Unpack(invalidMarker)
	if err == nil {
		t.Errorf("Expected error for invalid marker, got none")
	}
}
