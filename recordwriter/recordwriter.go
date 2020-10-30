// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

type RecordWriter interface {
	WriteHeader(s string)
	FinalizeHeader()
	Write(s string)
	WriteByte(s []byte)
	WriteSlice(val []string)
	// WriteQuoted(s string)
	// Next writes a comma
	Next()
	// Skip writes a comma unless it's the last
	Skip()
	EOR(columns int)
	Close()
	BeginArray(key string)
	CloseArray(key string)
}
