// goadifstat: check statistics of ADIF ADI files
// by Kenji Rikitake, JJ1BDX
// Usage: goadifstat [-f infile] [-o outfile] [-q query type]
// Valid query types: bands, country, dxcc, gridsquare, modes, nqso, submodes

package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/jj1bdx/adifparser"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

var ErrNoSuchField = adifparser.ErrNoSuchField

var bandList = []string{
	"2190M", "630M", "560M", "160M", "80M", "60M",
	"40M", "30M", "20M", "17M", "15M", "12M",
	"10M", "6M", "5M", "4M", "2M", "1.25M",
	"70CM", "33CM", "23CM", "13CM", "9CM", "6CM",
	"3CM", "1.25CM", "6MM", "4MM", "2.5MM", "2MM",
	"1MM"}

var mapBand map[string]int
var mapCountry map[string]int
var mapDxcc map[int]bool
var mapGrid map[string]bool
var mapMode map[string]int
var mapSubmode map[string]int

func initStatMaps() {
	mapBand = make(map[string]int)
	mapCountry = make(map[string]int)
	mapDxcc = make(map[int]bool)
	mapGrid = make(map[string]bool)
	mapMode = make(map[string]int)
	mapSubmode = make(map[string]int)
}

func updateStatMaps(record adifparser.ADIFRecord) {
	var err error
	var exists bool
	var key string
	var keynum int

	// band
	key, err = record.GetValue("band")
	if err != nil && err != ErrNoSuchField {
		fmt.Fprint(os.Stderr, err)
	} else {
		// Use uppercase for band names
		key = strings.ToUpper(key)
		_, exists = mapBand[key]
		if exists {
			mapBand[key]++
		} else {
			mapBand[key] = 1
		}
	}

	// country
	key, err = record.GetValue("country")
	if err != nil && err != ErrNoSuchField {
		fmt.Fprint(os.Stderr, err)
	} else {
		if key == "" {
			key = "(UNKNOWN)"
		} else {
			// Use uppercase for country names
			key = strings.ToUpper(key)
		}
		_, exists = mapCountry[key]
		if exists {
			mapCountry[key]++
		} else {
			mapCountry[key] = 1
		}
	}

	// dxcc
	key, err = record.GetValue("dxcc")
	if err != nil && err != ErrNoSuchField {
		fmt.Fprint(os.Stderr, err)
	} else if key != "" {
		// DXCC values are integers
		keynum, err = strconv.Atoi(key)
		if err != nil && err != ErrNoSuchField {
			fmt.Fprint(os.Stderr, err)
		} else {
			_, exists = mapDxcc[keynum]
			if !exists {
				mapDxcc[keynum] = true
			}
		}
	}

	// grid
	key, err = record.GetValue("gridsquare")
	if err != nil && err != ErrNoSuchField {
		fmt.Fprint(os.Stderr, err)
	} else if len(key) >= 4 {
		// Pick first four letters only
		key = key[0:4]
		// Grid locator first two letters are uppercase
		key = strings.ToUpper(key)
		_, exists = mapGrid[key]
		if !exists {
			mapGrid[key] = true
		}
	}

	// mode
	key, err = record.GetValue("mode")
	if err != nil && err != ErrNoSuchField {
		fmt.Fprint(os.Stderr, err)
	} else {
		key = strings.ToUpper(key)
		_, exists = mapMode[key]
		if exists {
			mapMode[key]++
		} else {
			mapMode[key] = 1
		}
	}

	// submode
	key, err = record.GetValue("submode")
	if err != nil && err != ErrNoSuchField {
		fmt.Fprint(os.Stderr, err)
	} else if key != "" {
		key = strings.ToUpper(key)
		_, exists = mapSubmode[key]
		if exists {
			mapSubmode[key]++
		} else {
			mapSubmode[key] = 1
		}
	}
}

func main() {
	var infile = flag.String("f", "", "input file (stdin if none)")
	var outfile = flag.String("o", "", "output file (stdout if none)")
	var query = flag.String("q", "", "query type")
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

	var writefp *os.File
	var writer *bufio.Writer
	if *outfile != "" {
		writefp, err = os.Create(*outfile)
		writer = bufio.NewWriter(writefp)
	} else {
		writefp = nil
		writer = bufio.NewWriter(os.Stdout)
	}

	initStatMaps()

	reader := adifparser.NewADIFReader(fp)
	for record, err := reader.ReadRecord(); 
	    record != nil || err != nil; 
	    record, err = reader.ReadRecord() {
		if err != nil {
			if err != io.EOF {
				fmt.Fprint(os.Stderr, err)
			}
			break // when io.EOF break the loop!
		}
		updateStatMaps(record)
	}

	// Calculate and output the stats
	switch {
	case *query == "bands":
		for band := range bandList {
			num, exists := mapBand[bandList[band]]
			if exists {
				fmt.Fprintf(writer, "%s %d ", bandList[band], num)
			}
		}
		fmt.Fprintf(writer, "\n")
	case *query == "country":
		keys := make([]string, 0, len(mapCountry))
		for k := range mapCountry {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(writer, "%s: %d\n", k, mapCountry[k])
		}
		fmt.Fprintln(writer, "(TOTAL):", reader.RecordCount())
	case *query == "dxcc":
		keys := make([]int, 0, len(mapDxcc))
		for k := range mapDxcc {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, n := range keys {
			fmt.Fprintf(writer, "%d ", n)
		}
		fmt.Fprintf(writer, "\n")
	case *query == "gridsquare":
		keys := make([]string, 0, len(mapGrid))
		for k := range mapGrid {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, g := range keys {
			fmt.Fprintf(writer, "%s ", g)
		}
		fmt.Fprintf(writer, "\n")
	case *query == "modes":
		keys := make([]string, 0, len(mapMode))
		for k := range mapMode {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(writer, "%s %d ", k, mapMode[k])
		}
		fmt.Fprintf(writer, "\n")
	case *query == "submodes":
		keys := make([]string, 0, len(mapSubmode))
		for k := range mapSubmode {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(writer, "%s %d ", k, mapSubmode[k])
		}
		fmt.Fprintf(writer, "\n")
	case *query == "nqso":
		fmt.Fprintln(writer, reader.RecordCount())
	default:
		fmt.Fprintln(os.Stderr, "Not a valid query type")
		fmt.Fprintln(os.Stderr, "Valid types:")
		fmt.Fprintln(os.Stderr, "  bands, country, dxcc, gridsquare,")
		fmt.Fprintln(os.Stderr, "  modes, nqso, submodes")
	}

	// Flush and close output here
	writer.Flush()
	if writefp != nil {
		writefp.Close()
	}

}
