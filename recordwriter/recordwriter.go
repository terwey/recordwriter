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

func debug(format string, i ...interface{}) {
	if DebugLog {
		fmt.Fprintf(os.Stderr, "recordwriter (debug): "+format+"\n", i...)
	}
}

func print(format string, i ...interface{}) {
	fmt.Fprintf(os.Stderr, "recordwriter: "+format+"\n", i...)
}

type RecordWriter interface {
	// WriteHeader append the string to the header row
	WriteHeader(s string)
	// FinalizeHeader signals the last header item has been set
	FinalizeHeader()
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
	Close() error
	BeginArray(key string)
	CloseArray(key string)
	WriteNull()

	Merge(ready <-chan string, wg *sync.WaitGroup, duration chan time.Duration)
}
