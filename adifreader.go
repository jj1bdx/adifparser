package adifparser

import (
	"bufio"
	"bytes"
	// "fmt"
	"io"
	// "os"
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
	// Version of the adif file
	version float64
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

func (ardr *baseADIFReader) ReadRecord() (ADIFRecord, error) {
	if !ardr.headerRead {
		ardr.readHeader()
	}
	buf, err := ardr.readRecord()
	if err != nil {
		if err != io.EOF {
			adiflog.Printf("readRecord: %v", err)
		}
		return nil, err
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		// No data left
		return nil, io.EOF
	}
	record, err := ParseADIFRecord(buf)
	if err == nil {
		ardr.records += 1
		return record, nil
	}
	return record, err
}

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
	ardr.version = 2
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
	ardr.headerRead = true
	eoh := []byte("<eoh>")
	adif_version := []byte("<adif_ver:")
	chunk, err := ardr.readChunk()
	if err != nil {
		// TODO: Log the error somewhere
		return
	}
	if bytes.HasPrefix(chunk, []byte("<")) {
		if bytes.HasPrefix(bytes.ToLower(chunk), adif_version) {
			ver_len_str_end := bytes.Index(chunk, []byte(">"))
			ver_len_str := string(chunk[len(adif_version):ver_len_str_end])
			ver_len, err := strconv.Atoi(ver_len_str)
			if err != nil {
				adiflog.Fatal(err)
			}
			ver_len_end := ver_len_str_end + 1 + ver_len
			ardr.version, err = strconv.ParseFloat(
				string(chunk[ver_len_str_end+1:ver_len_end]), 0)
			excess := chunk[ver_len_end:]
			eoh_end := bIndexCI(excess, eoh) + len(eoh)
			excess = excess[eoh_end:]
			ardr.excess = excess[tagStartPos(excess):]
		} else if bytes.HasPrefix(bytes.ToLower(chunk), eoh) {
			eoh_end := bIndexCI(chunk, eoh) + len(eoh)
			ardr.excess = chunk[eoh_end:]
		} else {
			ardr.excess = chunk
		}
		return
	}
	for !bContainsCI(chunk, eoh) {
		newchunk, err := ardr.readChunk()
		if err != nil {
			// TODO: Log the error somewhere
			return
		}
		chunk = append(chunk, newchunk...)
	}
	offset := bIndexCI(chunk, eoh) + len(eoh)
	chunk = chunk[offset:]
	ardr.excess = chunk[tagStartPos(chunk):]
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
	// fmt.Fprintf(os.Stderr, "record buf: %s\n", string(buf))
	// fmt.Fprintf(os.Stderr, "record excess: %s\n", string(ardr.excess))
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
