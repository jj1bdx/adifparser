package adifparser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	// "os"
	"strconv"
	"strings"
)

// Public interface for ADIFRecords
type ADIFRecord interface {
	// Print as ADIF String
	ToString() string
	// Fingerprint for duplication detection
	Fingerprint() string
	// Setters and getters
	GetValue(string) (string, error)
	SetValue(string, string)
	// Get all of the present field names
	GetFields() []string
}

// Internal implementation for ADIFRecord
type baseADIFRecord struct {
	values map[string]string
}

type fieldData struct {
	name     string
	value    string
	typecode byte
	hasType  bool
}

// Errors
var NoData = errors.New("No data to parse.")
var NoSuchField = errors.New("No such field.")
var InvalidField = errors.New("Invalid field definition.")

// Create a new ADIFRecord from scratch
func NewADIFRecord() *baseADIFRecord {
	record := &baseADIFRecord{}
	record.values = make(map[string]string)
	return record
}

// Parse an ADIFRecord
func ParseADIFRecord(buf []byte) (*baseADIFRecord, error) {
	record := NewADIFRecord()

	if len(buf) == 0 {
		return nil, NoData
	}

	for len(buf) > 0 {
		var data *fieldData
		var err error
		// fmt.Fprintf(os.Stderr, "parse buf: %s\n", string(buf))
		if buf[0] == '/' && buf[1] == '/' { // Recognize comments and skip them.
			end_of_line := bytes.IndexByte(buf, 13)
			if end_of_line > -1 {
				buf = buf[end_of_line+1:]
			} else { // The comment ends the record so there's no end-of-line.
				buf = buf[len(buf):]
			}
		} else {
			data, buf, err = getNextField(buf)
			if err != nil {
				return nil, err
			}
			if data != nil {
				// TODO: accomodate types
				record.values[data.name] = data.value
			} else {
				// skip if no valid data found in buf
				buf = buf[len(buf):]
			}
		}
	}

	return record, nil
}

// Get the next field, return field data, leftover data, and optional error
func getNextField(buf []byte) (*fieldData, []byte, error) {
	data := &fieldData{}

	// Check if valid tag start marker found in buf
	nextStart := bytes.IndexByte(buf, '<')
	if nextStart == -1 {
		return nil, buf, nil
	}
	// Extract name
	start_of_name := nextStart + 1
	end_of_name := bytes.IndexByte(buf, ':')
	end_of_tag := bytes.IndexByte(buf, '>')
	if end_of_name == -1 ||
		end_of_tag < end_of_name || end_of_name < start_of_name {
		// fmt.Fprintf(os.Stderr, "InvalidField:\n")
		// fmt.Fprintf(os.Stderr, "buf: %s\n", string(buf))
		// fmt.Fprintf(os.Stderr, "values: %d %d %d\n",
		//   start_of_name, end_of_name, end_of_tag)
		// fmt.Fprintf(os.Stderr, "values: %c %c %c\n",
		//    buf[start_of_name], buf[end_of_name], buf[end_of_tag])
		return nil, buf, InvalidField
	}
	// fmt.Fprintf(os.Stderr, "buf: %s\n", string(buf))
	data.name = strings.ToLower(string(buf[start_of_name:end_of_name]))
	buf = buf[end_of_name+1:]
	// Adjust to new buffer
	end_of_tag -= end_of_name + 1

	// Length
	var length int
	var err error
	start_type := bytes.IndexByte(buf, ':')
	if start_type == -1 || start_type > end_of_tag {
		end_of_length := bytes.IndexByte(buf, '>')
		length, err = strconv.Atoi(string(buf[:end_of_length]))
		buf = buf[end_of_length+1:]
		data.hasType = false
	} else {
		length, err = strconv.Atoi(string(buf[:start_type]))
		data.typecode = buf[start_type+1]
		buf = buf[start_type+3:]
		data.hasType = true
	}
	if err != nil {
		// TODO: log the error
		return nil, buf, err
	}

	// Value
	data.value = string(buf[:length])
	buf = bytes.TrimSpace(buf[length:])

	return data, buf, nil
}

func serializeField(name string, value string) string {
	return fmt.Sprintf("<%s:%d>%s", name, len(value), value)
}

// Print an ADIFRecord as a string
func (r *baseADIFRecord) ToString() string {
	var record bytes.Buffer
	for _, n := range ADIFfieldOrder {
		if v, ok := r.values[n]; ok {
			record.WriteString(serializeField(n, v))
		}
	}
	// Handle custom fields
	for n, v := range r.values {
		if !isStandardADIFField(n) {
			record.WriteString(serializeField(n, v))
		}
	}
	return record.String()
}

// Get fingerprint of ADIFRecord
func (r *baseADIFRecord) Fingerprint() string {
	fpfields := []string{
		"call", "station_callsign", "band",
		"freq", "mode", "qso_date", "time_on",
		"time_off"}
	fpvals := make([]string, 0, len(fpfields))
	for _, f := range fpfields {
		if n, ok := r.values[f]; ok {
			fpvals = append(fpvals, n)
		}
	}
	fptext := strings.Join(fpvals, "|")
	h := sha256.New()
	h.Write([]byte(fptext))
	return hex.EncodeToString(h.Sum(nil))
}

// Get a value
func (r *baseADIFRecord) GetValue(name string) (string, error) {
	if v, ok := r.values[name]; ok {
		return v, nil
	}
	return "", NoSuchField
}

// Set a value
func (r *baseADIFRecord) SetValue(name string, value string) {
	r.values[strings.ToLower(name)] = value
}

// Get all of the present field names
func (r *baseADIFRecord) GetFields() []string {
	keys := make([]string, len(r.values))
	i := 0
	for k := range r.values {
		keys[i] = k
		i++
	}
	return keys
}
