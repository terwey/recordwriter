// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	DebugLog = false
	Overflow = false
)

func print(format string, i ...interface{}) {
	fmt.Fprintf(os.Stderr, "recordwriter: "+format+"\n", i...)
}

type RecordWriter interface {
	// WriteHeader append the string to the header row
	WriteHeader(s string)
	// FinalizeHeader signals the last header item has been set
	FinalizeHeader() []string
	// Write append the string as a column to the row
	Write(s string)
	// WriteByte append the bytes as a column to the row
	WriteByte(s []byte)
	WriteSlice(val []string)
	// Next writes a comma
	Next()
	// Skip writes a comma unless it's the last
	Skip()
	// EOR finalizes the row
	EOR(columns int)
	EORMulti()
	Close() error
	BeginArray(key string)
	CloseArray(key string)
	WriteNull()
	Multibyte(out []byte, pos []uint32, columns int)

	Merge(ready <-chan string, wg *sync.WaitGroup, duration chan time.Duration)
}
