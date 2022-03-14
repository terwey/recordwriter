// SPDX-License-Identifier: MIT
// Copyright (c) 2022 Yorick Terweijden

//go:build debug

package recordwriter

import (
	"fmt"
	"os"
)

func debug(format string, i ...interface{}) {
	if DebugLog {
		fmt.Fprintf(os.Stderr, "recordwriter (debug): "+format+"\n", i...)
	}
}

func (rw *SQLiteWriter) appendToBuf(val interface{}) {
	if DebugLog {
		rw.debugBuf = append(rw.debugBuf, val)
	}
}
