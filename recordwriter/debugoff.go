// SPDX-License-Identifier: MIT
// Copyright (c) 2022 Yorick Terweijden

//go:build !debug

package recordwriter

func debug(format string, i ...interface{}) {
}

func (rw *SQLiteWriter) appendToBuf(val interface{}) {
}
