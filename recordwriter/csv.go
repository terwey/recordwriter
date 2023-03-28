// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/terwey/csv2sqlite/characters"
	"golang.org/x/term"
)

const (
	None int = 1 << iota
	Required
	SkipEmpty
	All
)

type CSVWriter struct {
	characters.Config
	origW            io.Writer
	w                bufio.Writer
	lastItemWasWrite bool
	count            int
	enclosureSet     int
	array            map[string][]string
	arrayKey         string
	writeToArray     bool
	path             string

	finalizedHeader []string

	buffer *bytes.Buffer
}

func NewCSVWriter(config characters.Config, w io.Writer, path string) *CSVWriter {
	csv := &CSVWriter{
		origW:  w,
		path:   path,
		Config: config,
	}

	csv.w = *bufio.NewWriterSize(w, 4*1024*1024)
	csv.array = make(map[string][]string)
	csv.buffer = bytes.NewBuffer(make([]byte, 4*1024*1024))

	return csv
}

func (csv *CSVWriter) SetConfig(config characters.Config) {
	csv.Config = config
}

func (csv *CSVWriter) SetEnclosureSet(set int) {
	csv.enclosureSet = set
}

func (csv *CSVWriter) writeString(s string) {
	csv.w.Write([]byte(s))
}

func (csv *CSVWriter) Write(s string) {
	if csv.writeToArray {
		csv.array[csv.arrayKey] = append(csv.array[csv.arrayKey], s)
		return
	}

	csv.w.Write(csv.preprocess([]byte(s)))

	csv.count++
	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) enclosing(s []byte) bool {
	if csv.enclosureSet == All {
		return true
	}

	contains := csv.needsEnclosure([]byte(s))

	if csv.enclosureSet != None {
		switch csv.enclosureSet {
		case Required:
			if contains {
				return true
			}
		case SkipEmpty:
			if len(s) == 0 {
				return false
			}
			return true
		case All:
			return true
		}

	}

	return false
}

func (csv *CSVWriter) preprocess(s []byte) []byte {
	csv.buffer.Reset()

	enclosing := csv.enclosing(s)

	if enclosing {
		csv.buffer.Write(csv.Config.Enclosure)
	}

	if bytes.Contains(s, csv.Config.Enclosure) {
		csv.buffer.Write(bytes.ReplaceAll(s, csv.Config.Enclosure, csv.Config.DoubleEnclosure))
	} else {
		csv.buffer.Write(s)
	}

	if enclosing {
		csv.buffer.Write(csv.Config.Enclosure)
	}

	return csv.buffer.Bytes()
}

func (csv *CSVWriter) needsEnclosure(s []byte) bool {
	contains := bytes.Contains([]byte(s), csv.Config.Delimiter)
	if !contains {
		for _, nl := range characters.KnownNewlines() {
			if bytes.Contains([]byte(s), nl) {
				contains = true
				break
			}
		}
	}
	if !contains {
		contains = bytes.Contains([]byte(s), csv.Config.Enclosure)
	}

	return contains
}

func (csv *CSVWriter) WriteByte(s []byte) {
	csv.w.Write(csv.preprocess(s))

	csv.count++
	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) WriteSlice(val []string) {
	if csv.writeToArray {
		csv.array[csv.arrayKey] = append(csv.array[csv.arrayKey], val...)
		return
	}

	if csv.enclosureSet == All {
		csv.w.Write(csv.Config.Enclosure)
	}

	for _, s := range val {
		csv.w.WriteString(s)
	}

	if csv.enclosureSet == All {
		csv.w.Write(csv.Config.Enclosure)
	}
	csv.count++
	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) WriteHeader(s string) {
	csv.Write(s)
	csv.count++
	csv.lastItemWasWrite = true
	csv.finalizedHeader = append(csv.finalizedHeader, s)
}

func (csv *CSVWriter) BeginArray(key string) {
	csv.arrayKey = key
	csv.writeToArray = true
}

func (csv *CSVWriter) CloseArray(key string) {
	csv.writeToArray = false
	if csv.enclosureSet == All {
		csv.w.Write(csv.Config.Enclosure)
	}
	csv.writeString(strings.Join(csv.array[key], `,`))
	if csv.enclosureSet == All {
		csv.w.Write(csv.Config.Enclosure)
	}
	csv.count++
	csv.array[key] = make([]string, 0)

	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) Next() {
	csv.w.Write(csv.Config.Delimiter)
	csv.lastItemWasWrite = false
}

func (csv *CSVWriter) Skip() {
	if !csv.lastItemWasWrite {
		csv.count++
		if csv.enclosureSet == All {
			csv.w.Write(csv.Config.DoubleEnclosure)
		}
	}
	csv.lastItemWasWrite = false
}

func (csv *CSVWriter) WriteNull() {
	csv.count++
	csv.lastItemWasWrite = true
	if csv.enclosureSet == All {
		csv.w.Write(csv.Config.Enclosure)
		csv.w.Write(csv.Config.Enclosure)
	}
}

func (csv *CSVWriter) FinalizeHeader() []string {
	csv.EOR(csv.count)
	csv.w.Flush()

	if f, ok := csv.origW.(*os.File); ok {
		if !term.IsTerminal(int(f.Fd())) {
			err := f.Sync()
			if err != nil {
				log.Printf("cannot sync file: %s", err)
			}
		}
	}

	return csv.finalizedHeader
}

func (csv *CSVWriter) EOR(columns int) {
	// log.Printf("finalizing row: columns %d (count: %d)", columns, csv.count)
	// finalize the columns
	if csv.count < columns {
		for csv.count <= columns {
			// log.Printf("csv.count %d < columns %d", csv.count, columns)
			csv.Skip()
			if csv.count != columns {
				// log.Printf("csv.count %d != columns %d", csv.count, columns)
				csv.Next()
			}
		}
	}
	csv.w.Write(csv.Config.EOL)
	csv.lastItemWasWrite = false
	csv.count = 0

	csv.w.Flush()
}

func (csv *CSVWriter) Close() error {
	// log.Printf("close(): %s", csv.path)
	csv.w.Flush()

	// we must sync the data on close
	if f, ok := csv.origW.(*os.File); ok {
		if !term.IsTerminal(int(f.Fd())) {
			err := f.Sync()
			if err != nil {
				log.Printf("cannot sync file: %s", err)
			}

			err = f.Close()
			if err != nil {
				log.Printf("cannot close file: %s", err)
			}
		}
	}

	return nil
}

func (csv *CSVWriter) GetPath() string {
	return csv.path
}

func (csv *CSVWriter) Merge(ready <-chan string, wg *sync.WaitGroup, duration chan time.Duration) {
	defer wg.Done()
	started := false
	var start time.Time

	// when writing to stdout there is no path
	if csv.path == "" {
		for range ready {
			if !started {
				started = true
				start = time.Now()
			}
		}
		duration <- time.Since(start)
		return
	}

	var mergeWg sync.WaitGroup

	dst, err := os.OpenFile(csv.path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		panic(err)
	}

	mu := &sync.Mutex{}
	buf := make([]byte, 4*1024*1024)

	for {
		target, more := <-ready
		if !more {
			break
		}
		if !started {
			started = true
			start = time.Now()
		}

		mergeWg.Add(1)
		go func(dst *os.File, target string, w *sync.WaitGroup) {
			defer w.Done()

			mu.Lock()
			defer mu.Unlock()

			src, err := os.Open(target)
			if err != nil {
				panic(err)
			}

			_, err = io.CopyBuffer(dst, src, buf)
			// log.Printf("n: %d", n)

			if err != nil {
				log.Printf("err: %s", err)
			}

			dst.Sync()
			src.Close()

			dst.Seek(0, io.SeekEnd)

			go os.Remove(target)

			// log.Printf("dst: %s, target: %s", csv.path, target)
		}(dst, target, &mergeWg)
	}
	mergeWg.Wait()
	duration <- time.Since(start)
}

func (csv *CSVWriter) Multibyte(in []byte, pos []uint32, columns int) {
	var prev uint32 = 0
	for i, dlen := range pos {
		if len(in[prev:prev+dlen]) == 0 {
			csv.Skip()
		} else {
			csv.WriteByte(in[prev : prev+dlen])
		}

		if i != len(pos)-1 {
			csv.Next()
		}

		prev += dlen
	}

	csv.EOR(len(pos))
}

func (csv *CSVWriter) EORMulti() {
	// noop
}
