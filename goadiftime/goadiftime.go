package main

import (
	"flag"
	"fmt"
	"github.com/jj1bdx/adifparser"
	"io"
	"os"
	"sort"
	"strconv"
	"time"
)

type recordWithTime struct {
	date   time.Time
	record adifparser.ADIFRecord
}

func main() {
	var infile = flag.String("f", "", "input file ('-' for stdin)")
	var outfile = flag.String("o", "", "output file (stdout if none)")
	var reverse bool
	flag.BoolVar(&reverse, "r", false, "reverse sort (new to old)")

	var fp *os.File
	var err error

	records := []recordWithTime{}

	flag.Parse()

	if *infile == "" {
		fmt.Fprint(os.Stderr, "Need infile.\n")
		return
	}

	if *infile == "-" {
		fp = os.Stdin
	} else {
		fp, err = os.Open(*infile)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			return
		}
	}

	var writefp *os.File
	if *outfile != "" {
		writefp, err = os.Create(*outfile)
	} else {
		writefp = os.Stdout
	}

	fmt.Fprintf(os.Stderr, "reverse: %d\n", reverse)

	reader := adifparser.NewDedupeADIFReader(fp)
	for record, err := reader.ReadRecord(); record != nil || err != nil; record, err = reader.ReadRecord() {
		if err != nil {
			if err != io.EOF {
				fmt.Fprint(os.Stderr, err)
			}
			break // when io.EOF break the loop!
		}

		adifdate, _ := record.GetValue("qso_date")
		adiftime, _ := record.GetValue("time_on")

		adifyear, _ := strconv.Atoi(adifdate[0:4])
		adifmonth, _ := strconv.Atoi(adifdate[4:6])
		adifday, _ := strconv.Atoi(adifdate[6:8])
		adifhour, _ := strconv.Atoi(adiftime[0:2])
		adifminute, _ := strconv.Atoi(adiftime[2:4])
		adifsecond := 0
		if len(adiftime) > 4 {
			adifsecond, _ = strconv.Atoi(adiftime[4:6])
		}
		recordtime := time.Date(
			adifyear, time.Month(adifmonth), adifday,
			adifhour, adifminute, adifsecond,
			0, time.UTC)

		recordandtime := recordWithTime{recordtime, record}
		records = append(records, recordandtime)
	}

	if reverse {
		sort.Slice(records,
			func(i, j int) bool {
				return records[i].date.After(records[j].date)
			})
	} else {
		sort.Slice(records,
			func(i, j int) bool {
				return records[i].date.Before(records[j].date)
			})
	}

	for i := range records {
		fmt.Fprintln(writefp, records[i].record.ToString())
	}

	// Close output here
	if writefp != os.Stdout {
		writefp.Close()
	}

}
