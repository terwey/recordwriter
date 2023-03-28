// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Yorick Terweijden

package recordwriter

import (
	"sync"
	"time"
)

type DropWriter struct{}

func NewDropWriter() *DropWriter {
	return &DropWriter{}
}

// WriteHeader append the string to the header row
func (d *DropWriter) WriteHeader(s string) {
}

// FinalizeHeader signals the last header item has been set
func (d *DropWriter) FinalizeHeader() []string {
	return nil
}

// Write append the string as a column to the row
func (d *DropWriter) Write(s string) {}

// WriteByte append the bytes as a column to the row
func (d *DropWriter) WriteByte(s []byte) {
	// io.Discard.Write(s)
}
func (d *DropWriter) WriteSlice(val []string) {}

// Next writes a comma
func (d *DropWriter) Next() {}

// Skip writes a comma unless it's the last
func (d *DropWriter) Skip() {}

// EOR finalizes the row
func (d *DropWriter) EOR(columns int) {}
func (d *DropWriter) Close() error {
	return nil
}
func (d *DropWriter) BeginArray(key string) {}
func (d *DropWriter) CloseArray(key string) {}
func (d *DropWriter) WriteNull()            {}
func (d *DropWriter) GetPath() string {
	return ""
}

func (d *DropWriter) Merge(ready <-chan string, wg *sync.WaitGroup, duration chan time.Duration) {
	defer wg.Done()
	started := false
	var start time.Time
	for range ready {
		if !started {
			started = true
			start = time.Now()
		}

		// io.Discard.Write([]byte(target))
	}
	duration <- time.Since(start)
}

func (d *DropWriter) Multibyte(out []byte, pos []uint32, columns int) {}

func (d *DropWriter) EORMulti() {}
