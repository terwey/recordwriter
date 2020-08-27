// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteWriter struct {
	db               *sql.DB
	tx               *sql.Tx
	stmt             *sql.Stmt
	txStmt           *sql.Stmt
	header           []string
	buf              []interface{}
	count            int
	rowCount         int
	size             int
	maxSize          int
	lastItemWasWrite bool
}

func NewSQLWriter(path string, size int) *SQLiteWriter {
	var err error
	rw := &SQLiteWriter{}
	os.Remove(path)
	// rw.db, err = sql.Open("sqlite3", fmt.Sprintf("%s?_journal=off&_sync=off&_locking=EXCLUSIVE", path))
	rw.db, err = sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal(err)
	}
	rw.maxSize = size

	return rw
}

func (rw *SQLiteWriter) WriteHeader(s string) {
	rw.header = append(rw.header, s)
}

func (rw *SQLiteWriter) FinalizeHeader() {
	// for compatibility with the csv writer we need to reset the buf
	rw.buf = make([]interface{}, 0)
	var err error
	_, err = rw.db.Exec("DROP TABLE IF EXISTS data")
	if err != nil {
		log.Fatal(err)
	}

	_, err = rw.db.Exec(fmt.Sprintf("CREATE TABLE data (%s)",
		strings.Join(rw.getCreateColumns(), `,`)))
	if err != nil {
		log.Fatal(err)
	}
	query := fmt.Sprintf("INSERT INTO data (%s) VALUES (%s)",
		strings.Join(rw.header, `,`),
		strings.Join(rw.getQuestionmarks(), `,`))

	rw.stmt, err = rw.db.Prepare(query)
	if err != nil {
		log.Fatal(err)
	}
	rw.tx, err = rw.db.Begin()
	if err != nil {
		log.Fatal(err)
	}
}

func (rw *SQLiteWriter) Write(s string) {
	rw.buf = append(rw.buf, s)
	rw.count++
	rw.lastItemWasWrite = true
}
func (rw *SQLiteWriter) WriteQuoted(s string) {
	rw.buf = append(rw.buf, s)
	rw.count++
	rw.lastItemWasWrite = true
}

func (rw *SQLiteWriter) Next() {
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) Skip() {
	if !rw.lastItemWasWrite {
		rw.buf = append(rw.buf, nil)
		rw.count++
	}
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) EOR(columns int) {
	var err error
	if rw.txStmt == nil {
		rw.txStmt = rw.tx.Stmt(rw.stmt)
	}

	// we're gonna guestimate the size here
	if rw.size == 0 {
		var guess int
		for _, v := range rw.buf {
			if s, ok := v.(string); ok {
				guess += len(s)
			}
		}
		rw.size = rw.maxSize / guess
	}

	if rw.count < columns {
		for rw.count < columns {
			rw.buf = append(rw.buf, nil)
			rw.count++
		}
	}

	_, err = rw.txStmt.Exec(rw.buf...)
	if err != nil {
		log.Fatal(err)
	}
	rw.buf = make([]interface{}, 0, len(rw.header))
	if rw.rowCount > rw.size {
		if err = rw.tx.Commit(); err != nil {
			log.Fatal(err)
		}
		rw.tx, err = rw.db.Begin()
		if err != nil {
			log.Fatal(err)
		}
		rw.txStmt = rw.tx.Stmt(rw.stmt)
		rw.rowCount = 0
	}

	rw.count = 0
	rw.rowCount++
}

func (rw *SQLiteWriter) Close() {
	if err := rw.tx.Commit(); err != nil {
		log.Fatal(err)
	}

	if err := rw.db.Close(); err != nil {
		log.Fatal(err)
	}
}

func (rw *SQLiteWriter) getCreateColumns() []string {
	var buf []string

	for _, v := range rw.header {
		buf = append(buf, v+` TEXT`)
	}
	return buf
}

func (rw *SQLiteWriter) getQuestionmarks() []string {
	var qm []string
	for range rw.header {
		qm = append(qm, `?`)
	}

	return qm
}
