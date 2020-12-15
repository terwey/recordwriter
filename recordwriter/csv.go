// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

import (
	"io"
	"strings"
)

type CSVWriter struct {
	w                io.StringWriter
	lastItemWasWrite bool
	count            int
	sep              string
	quote            string
	quoteAll         bool
	array            map[string][]string
	arrayKey         string
	writeToArray     bool
}

func NewCSVWriter(w io.StringWriter) *CSVWriter {
	csv := &CSVWriter{
		w:        w,
		sep:      `,`,
		quote:    `"`,
		quoteAll: true,
	}

	csv.array = make(map[string][]string)

	return csv
}

func (csv *CSVWriter) SetSeperator(s string) {
	csv.sep = s
}

func (csv *CSVWriter) SetQuote(s string) {
	csv.quote = s
}

func (csv *CSVWriter) QuoteAll(enabled bool) {
	csv.quoteAll = enabled
}

func (csv *CSVWriter) Write(s string) {
	if csv.writeToArray {
		csv.array[csv.arrayKey] = append(csv.array[csv.arrayKey], s)
		return
	}
	if csv.quoteAll {
		csv.w.WriteString(csv.quote)
	}

	csv.w.WriteString(s)
	if csv.quoteAll {
		csv.w.WriteString(csv.quote)
	}
	csv.count++
	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) WriteByte(s []byte) {
	csv.Write(string(s))
}

func (csv *CSVWriter) WriteSlice(val []string) {
	if csv.writeToArray {
		csv.array[csv.arrayKey] = append(csv.array[csv.arrayKey], val...)
		return
	}

	if csv.quoteAll {
		csv.w.WriteString(csv.quote)
	}

	for _, s := range val {
		csv.w.WriteString(s)
	}

	if csv.quoteAll {
		csv.w.WriteString(csv.quote)
	}
	csv.count++
	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) WriteHeader(s string) {
	csv.Write(s)
	csv.count++
	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) BeginArray(key string) {
	csv.arrayKey = key
	csv.writeToArray = true
}

func (csv *CSVWriter) CloseArray(key string) {
	csv.writeToArray = false
	if csv.quoteAll {
		csv.w.WriteString(csv.quote)
	}
	csv.w.WriteString(strings.Join(csv.array[key], `,`))
	if csv.quoteAll {
		csv.w.WriteString(csv.quote)
	}
	csv.count++
	csv.array[key] = make([]string, 0)

	csv.lastItemWasWrite = true
}

func (csv *CSVWriter) Next() {
	csv.w.WriteString(csv.sep)
	csv.lastItemWasWrite = false
}

func (csv *CSVWriter) Skip() {
	if !csv.lastItemWasWrite {
		csv.count++
		if csv.quoteAll {
			csv.w.WriteString(csv.quote)
			csv.w.WriteString(csv.quote)
		}
	}
	csv.lastItemWasWrite = false
}

func (csv *CSVWriter) FinalizeHeader() {
	csv.EOR(csv.count)
}

func (csv *CSVWriter) EOR(columns int) {
	// finalize the columns
	if csv.count < columns {
		for csv.count < columns {
			csv.Skip()
			if csv.count != columns {
				csv.Next()
			}
		}
	}
	csv.w.WriteString("\n")
	csv.lastItemWasWrite = false
	csv.count = 0
}

func (csv *CSVWriter) Close() error {
	return nil
}
