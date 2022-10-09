package main

import (
	"flag"
	"fmt"
	"github.com/jj1bdx/adifparser"
	"io"
	"os"
)

func main() {
	var infile = flag.String("infile", "", "Input file.")
	var outfile = flag.String("outfile", "", "Output file.")

	flag.Parse()

	if *infile == "" {
		fmt.Fprint(os.Stderr, "Need infile.\n")
		return
	}

	fp, err := os.Open(*infile)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return
	}

	var writefp *os.File
	if *outfile != "" {
		writefp, err = os.Create(*outfile)
	} else {
		writefp = os.Stdout
	}

	reader := adifparser.NewDedupeADIFReader(fp)
	for record, err := reader.ReadRecord(); record != nil || err != nil; record, err = reader.ReadRecord() {
		if err != nil {
			if err != io.EOF {
				fmt.Fprint(os.Stderr, err)
			}
			break // when io.EOF break the loop!
		}
		fmt.Fprintln(writefp, record.ToString())
	}

	if writefp != os.Stdout {
		writefp.Close()
	}
	fmt.Fprintf(os.Stderr, "Total records: %d\n", reader.RecordCount())
}
