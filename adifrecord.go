package adifparser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
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
	// Delete a field
	DeleteField(string) (bool, error)
}

// Internal implementation for ADIFRecord
type baseADIFRecord struct {
	values map[string]string
}

// Errors
var ErrNoSuchField = errors.New("no such field")

// Create a new ADIFRecord from scratch
func NewADIFRecord() *baseADIFRecord {
	record := &baseADIFRecord{}
	record.values = make(map[string]string)
	return record
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
	// Pick up custom field names as keys first
	custom_keys := make([]string, 0, len(r.values))
	for n := range r.values {
		if !isStandardADIFField(n) {
			custom_keys = append(custom_keys, n)
		}
	}
	// Sort the custom keys
	sort.Strings(custom_keys)
	// Print the custom fields with sorted keys
	for _, k := range custom_keys {
		record.WriteString(serializeField(k, r.values[k]))
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
	return "", ErrNoSuchField
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

// Delete a field (from the internal map)
func (r *baseADIFRecord) DeleteField(name string) (bool, error) {
	if _, ok := r.values[name]; ok {
		delete(r.values, name)
		return true, nil
	}
	return false, ErrNoSuchField
}
