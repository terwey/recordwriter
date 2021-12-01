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
func (d *DropWriter) FinalizeHeader() {}

// Write append the string as a column to the row
func (d *DropWriter) Write(s string) {}

// WriteByte append the bytes as a column to the row
func (d *DropWriter) WriteByte(s []byte)      {}
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

// TODO: dummy method
func (d *DropWriter) Merge(ready <-chan string, wg *sync.WaitGroup, duration chan time.Duration) {
	defer wg.Done()
	started := false
	var start time.Time
	for range ready {
		if !started {
			started = true
			start = time.Now()
		}
	}
	duration <- time.Since(start)
}
