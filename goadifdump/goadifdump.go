// goadifdump: reformat preserve all ADIF file fields without deduping
// by Kenji Rikitake, JJ1BDX
// Usage: goadifdump [-f infile] [-o outfile]
//
// This is a skeleton code set for adding further processing
//
// Coding convention:
// Use reader for reading each record (with ADIFReader)
// Use writer for writing each record (with ADIFWriter)

package main

import (
	"flag"
	"fmt"
	"github.com/jj1bdx/adifparser"
	"io"
	"os"
)

func main() {
	var infile = flag.String("f", "", "input file (stdout in none)")
	var outfile = flag.String("o", "", "output file (stdout if none)")

	var fp *os.File
	var err error

	flag.Parse()

	if *infile == "" {
		fp = os.Stdin
	} else {
		fp, err = os.Open(*infile)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			return
		}
	}

	var writer adifparser.ADIFWriter
	var writefp *os.File
	if *outfile != "" {
		writefp, err = os.Create(*outfile)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			return
		}
		writer = adifparser.NewADIFWriter(writefp)
	} else {
		writefp = nil
		writer = adifparser.NewADIFWriter(os.Stdout)
	}

	// For deduping, use this filter API:
	// reader := adifparser.NewDedupeADIFReader(fp)

	reader := adifparser.NewADIFReader(fp)
	for record, err := reader.ReadRecord(); record != nil || err != nil; record, err = reader.ReadRecord() {
		if err != nil {
			if err != io.EOF {
				fmt.Fprint(os.Stderr, err)
			}
			break // when io.EOF break the loop!
		}

		// process things here with the record
		writer.WriteRecord(record)

	}

	if writefp != os.Stdout {
		writefp.Close()
	}
	fmt.Fprintf(os.Stderr, "Total records: %d\n", reader.RecordCount())
}
