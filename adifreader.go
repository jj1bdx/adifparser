package adifparser

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
)

// Interface for ADIFReader
type ADIFReader interface {
	ReadRecord() (ADIFRecord, error)
	RecordCount() int
}

// Real implementation of ADIFReader
type baseADIFReader struct {
	// Underlying bufio Reader
	rdr *bufio.Reader
	// Whether or not the header is included
	noHeader bool
	// Whether or not the header has been read
	headerRead bool
	// Version string of the adif file
	version string
	// Excess read data
	excess []byte
	// Record count
	records int
}

type dedupeADIFReader struct {
	baseADIFReader
	// Store seen entities
	seen map[string]bool
}

type elementData struct {
	// ADIF field name (in ASCII, set to lowercase)
	name string
	// ADIF field (if nil, only the field name exists)
	value string
	// ADIF data type indicator (optional, set to uppercase)
	typecode byte
	// ADIF specifier always has a corresponding value
	// If hasValue is false, string inside "<>" is
	// a tag without a value
	hasValue bool
	// ADIF specifier can optionally have a type
	hasType bool
	// Length of value bytes/string
	valueLength int
}

func (ardr *baseADIFReader) ReadRecord() (ADIFRecord, error) {
	record := NewADIFRecord()

	if !ardr.headerRead {
		ardr.readHeader()
	}

	foundeor := false
	for !foundeor {
		element, err := ardr.readElement()
		if err != nil {
			if err != io.EOF {
				adiflog.Printf("readElement: %v", err)
			}
			return nil, err
		}
		if element.name == "eor" && !element.hasValue {
			foundeor = true
			break
		}
		if element.hasValue {
			// TODO: accomodate types
			record.values[element.name] = element.value
		}
	}
	// Successfully parsed the record
	ardr.records++
	return record, nil
}

// Errors
var InvalidFieldLength = errors.New("Invalid field length.")
var TypeCodeExceedOneByte = errors.New("Type Code exceeds one byte.")
var UnknownColons = errors.New("Unknown colons in the tag.")

func (ardr *dedupeADIFReader) ReadRecord() (ADIFRecord, error) {
	for true {
		record, err := ardr.baseADIFReader.ReadRecord()
		if err != nil {
			return nil, err
		}
		fp := record.Fingerprint()
		if _, ok := ardr.seen[fp]; !ok {
			ardr.seen[fp] = true
			return record, nil
		}
	}
	return nil, nil
}

func NewADIFReader(r io.Reader) *baseADIFReader {
	reader := &baseADIFReader{}
	reader.init(r)
	return reader
}

func NewDedupeADIFReader(r io.Reader) *dedupeADIFReader {
	reader := &dedupeADIFReader{}
	reader.init(r)
	reader.seen = make(map[string]bool)
	return reader
}

func (ardr *baseADIFReader) init(r io.Reader) {
	ardr.rdr = bufio.NewReader(r)
	// Assumption
	ardr.version = "2.0"
	ardr.records = 0
	// check header
	filestart, err := ardr.rdr.Peek(1)
	if err != nil {
		// TODO: Log the error somewhere
		return
	}
	ardr.noHeader = filestart[0] == '<'
	// if header does not exist, header can be skipped
	// and treated as read
	ardr.headerRead = ardr.noHeader
}

func (ardr *baseADIFReader) readHeader() {
	foundeoh := false
	for !foundeoh {
		element, err := ardr.readElement()
		if err != nil {
			// TODO: Log the error somewhere
			return
		}
		if element.name == "eoh" && !element.hasValue {
			foundeoh = true
			break
		}
		if element.name == "adif_ver" && element.hasValue {
			ardr.version = element.value
		}
	}

	ardr.headerRead = true
}

func (ardr *baseADIFReader) readChunk() ([]byte, error) {
	chunk := make([]byte, 1024)
	n, err := ardr.rdr.Read(chunk)
	if err != nil {
		return nil, err
	}
	return chunk[:n], nil
}

func (ardr *baseADIFReader) readRecord() ([]byte, error) {
	eor := []byte("<eor>")
	buf := ardr.excess
	ardr.excess = nil
	for !bContainsCI(buf, eor) {
		newchunk, err := ardr.readChunk()
		if err != nil {
			ardr.excess = nil
			if err == io.EOF {
				buf = trimLotwEof(buf)
				// Expected, pass it up the chain
				if len(buf) > 0 {
					return bytes.Trim(buf, "\r\n"), nil
				}
				return nil, err
			}
			adiflog.Println(err)
			return nil, err
		}
		buf = append(buf, newchunk...)
	}
	buf = trimLotwEof(buf)
	record_end := bIndexCI(buf, eor)
	ardr.excess = buf[record_end+len(eor):]
	return bytes.Trim(buf[:record_end], "\r\n"), nil
}

func trimLotwEof(buf []byte) []byte {
	// LotW ends their files with a non-standard EOF tag.
	lotwEOF := []byte("<app_lotw_eof>")
	if eofIndex := bIndexCI(buf, lotwEOF); eofIndex != -1 {
		buf = buf[:eofIndex]
	}
	return buf
}

func (ardr *baseADIFReader) RecordCount() int {
	return ardr.records
}

func (ardr *baseADIFReader) readElement() (*elementData, error) {
	var c byte
	var err error
	var fieldname []byte
	var fieldvalue []byte
	var fieldtype byte
	var fieldlenstr []byte
	var fieldlength int = 0

	data := &elementData{}
	data.name = ""
	data.value = ""
	data.typecode = 0
	data.valueLength = 0

	// Look for "<" (open tag) first
	foundopentag := false
	for !foundopentag {
		// Read a byte (aka character)
		c, err = ardr.rdr.ReadByte()
		if err != nil {
			return nil, err
		}
		foundopentag = c == '<'
	}

	// Get field name
	data.hasValue = false
	data.hasType = false
	// Look for ">" (close tag) first
	foundclosetag := false
	foundcolonnum := 0
	foundtype := false
	for !foundclosetag {
		// Read a byte (aka character)
		c, err = ardr.rdr.ReadByte()
		if err != nil {
			return nil, err
		}
		foundclosetag = c == '>'
		if foundclosetag {
			break
		}
		switch foundcolonnum {
		case 0:
			if c == ':' {
				foundcolonnum++
				data.hasValue = true
				break
			} else {
				fieldname = append(fieldname, c)
				break
			}
		case 1:
			if c == ':' {
				foundcolonnum++
				data.hasType = true
				break
			} else {
				if c >= '0' && c <= '9' {
					fieldlenstr = append(fieldlenstr, c)
					break
				} else {
					return nil, InvalidFieldLength
				}
			}
		case 2:
			if !foundtype {
				fieldtype = c
				foundtype = true
				break
			} else {
				return nil, TypeCodeExceedOneByte
			}
		default:
			return nil, UnknownColons
		}
	}

	data.name = string(bStrictToLower(fieldname))
	data.typecode = charToUpper(fieldtype)

	// Get field length
	if data.hasValue {
		fieldlength, err = strconv.Atoi(string(fieldlenstr))
		if err != nil {
			return nil, err
		}
		data.valueLength = fieldlength

		// Get field value/content,
		// with the byte length specified by the field length
		for i := 0; i < fieldlength; i++ {
			c, err = ardr.rdr.ReadByte()
			if err != nil {
				return nil, err
			}
			fieldvalue = append(fieldvalue, c)
		}
		data.value = string(fieldvalue)
	}

	return data, nil
}
