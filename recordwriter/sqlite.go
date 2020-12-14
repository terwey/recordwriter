// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

/*
// SQLite compilation options.
#cgo CFLAGS: -DSQLITE_DEFAULT_PAGE_SIZE=32768
#cgo CFLAGS: -DSQLITE_MAX_VARIABLE_NUMBER=1000000
#cgo CFLAGS: -DSQLITE_DEFAULT_LOCKING_MODE=1
#cgo CFLAGS: -DSQLITE_DEFAULT_WAL_SYNCHRONOUS=0
*/

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
)

type SQLiteWriter struct {
	conn             *sqlite3.Conn
	stmt             *sqlite3.Stmt
	header           []string
	buf              []string
	count            int
	rowCount         int
	size             int
	lastItemWasWrite bool
	array            map[string][]string
	arrayKey         string
	writeToArray     bool
	path             string
}

func NewSQLWriter(path string, size int) *SQLiteWriter {
	var err error
	rw := &SQLiteWriter{}
	os.Remove(path)
	rw.conn, err = sqlite3.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	rw.size = size
	rw.array = make(map[string][]string)
	rw.path = path

	return rw
}

func (rw *SQLiteWriter) GetPath() string {
	return rw.path
}

func (rw *SQLiteWriter) WriteHeader(s string) {
	rw.header = append(rw.header, s)
}

func (rw *SQLiteWriter) FinalizeHeader() {
	// for compatibility with the csv writer we need to reset the buf
	// rw.buf = make([]interface{}, 0)
	rw.buf = make([]string, 0)
	var err error
	err = rw.conn.Exec(fmt.Sprintf("CREATE TABLE data (%s)",
		strings.Join(rw.getCreateColumns(), `,`)))
	if err != nil {
		log.Fatal(err)
	}

	rw.stmt, err = rw.conn.Prepare(rw.GetQuery())
	if err != nil {
		log.Fatalf("sqlite cannot prepare query, error: %s\nquery: %s", err, rw.GetQuery())
	}

	if err = rw.conn.Exec("PRAGMA locking_mode = EXCLUSIVE"); err != nil {
		log.Fatal(err)
	}

	if err = rw.conn.Exec("PRAGMA synchronous = 0"); err != nil {
		log.Fatal(err)
	}

	if err = rw.conn.Exec("PRAGMA mmap_size=468435456"); err != nil {
		log.Fatal(err)
	}

	if err = rw.conn.Exec("PRAGMA journal_mode = OFF"); err != nil {
		log.Fatal(err)
	}

	// if err = rw.conn.Exec("PRAGMA automatic_index = 0"); err != nil {
	// 	log.Fatal(err)
	// }

	err = rw.conn.BeginExclusive()
	if err != nil {
		log.Fatal(err)
	}
}

func (rw *SQLiteWriter) GetQuery() string {
	return fmt.Sprintf("INSERT INTO data (%s) VALUES (%s)",
		strings.Join(rw.header, `,`),
		strings.Join(rw.getQuestionmarks(), `,`))
}

func (rw *SQLiteWriter) Write(s string) {
	if rw.writeToArray {
		rw.array[rw.arrayKey] = append(rw.array[rw.arrayKey], s)
		return
	}

	rw.buf = append(rw.buf, s)
	rw.count++
	rw.lastItemWasWrite = true
}

// WriteByte uses an unsafe.Pointer method to return the incoming []byte as a string
// beware: could eat your dog
func (rw *SQLiteWriter) WriteByte(s []byte) {
	rw.buf = append(rw.buf, ByteSlice2String(s))
	rw.count++
	rw.lastItemWasWrite = true
}

func ByteSlice2String(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

func (rw *SQLiteWriter) WriteSlice(val []string) {
	if rw.writeToArray {
		rw.array[rw.arrayKey] = append(rw.array[rw.arrayKey], val...)
		return
	}

	for _, s := range val {
		rw.buf = append(rw.buf, s)
	}

	rw.count++
	rw.lastItemWasWrite = true
}

func (rw *SQLiteWriter) Next() {
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) Skip() {
	if !rw.lastItemWasWrite {
		// originally this wrote away a nil but the existing implementation writes an empty string
		rw.buf = append(rw.buf, ``)
		rw.count++
	}
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) EOR(columns int) {
	var err error

	if rw.count < columns {
		for rw.count < columns {
			rw.buf = append(rw.buf, "")
			rw.count++
		}
	}

	// assume default is one
	// err = rw.stmt.Exec(rw.buf...)
	err = rw.stmt.ExecStrings(rw.buf...)

	if err != nil {
		rw.handleError(err)
	}

	// log.Printf("Checking to flush")
	if rw.rowCount > rw.size {
		// log.Printf("Flushing: %d, bulkCount: %d", rw.rowCount, rw.bulkCount)
		if err = rw.conn.Commit(); err != nil {
			log.Fatal(err)
		}
		err = rw.conn.BeginExclusive()
		if err != nil {
			log.Fatal(err)
		}

		rw.rowCount = 0
	}

	// rw.buf = make([]interface{}, 0, len(rw.header))
	rw.buf = make([]string, 0, len(rw.header))

	rw.count = 0
	rw.rowCount++
}

func (rw *SQLiteWriter) handleError(err error) {
	header := rw.getCreateColumns()
	log.Printf("\n----------")
	log.Printf("Failed importing to SQLite")
	log.Printf("header (len: %d): \n----------\n%v", len(header), header)
	log.Printf("data (len: %d): \n----------\n", len(rw.buf))
	log.Printf("%s", rw.buf)
	log.Printf("%s", err)
	log.Printf("----------")
	log.Printf("Query: \n%s", rw.GetQuery())
	log.Printf("----------")
	log.Fatal("Ending")
}

func (rw *SQLiteWriter) Merge(ready <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	opened := false
	var err error
	for target := range ready {
		if !opened {
			rw.conn, err = sqlite3.Open(rw.path)
			if err != nil {
				log.Fatalf("Cannot open for merging (%s): %s", rw.path, err)
			}

			if err = rw.conn.Exec("PRAGMA locking_mode = EXCLUSIVE"); err != nil {
				log.Fatal(err)
			}

			if err = rw.conn.Exec("PRAGMA synchronous = 0"); err != nil {
				log.Fatal(err)
			}

			if err = rw.conn.Exec("PRAGMA mmap_size=468435456"); err != nil {
				log.Fatal(err)
			}

			if err = rw.conn.Exec("PRAGMA journal_mode = OFF"); err != nil {
				log.Fatal(err)
			}

			opened = true
		}

		// target := fmt.Sprintf("%s.%d", rw.path, i)
		err := rw.conn.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS db2", target))
		if err != nil {
			log.Fatalf("Cannot attach (%s): %s", target, err)
		}

		err = rw.conn.BeginExclusive()
		if err != nil {
			log.Fatalf("Cannot begin: %s", err)
		}

		err = rw.conn.Exec("INSERT INTO main.data SELECT * FROM db2.data")
		if err != nil {
			log.Fatalf("Cannot insert (%s): %s", target, err)
		}

		err = rw.conn.Commit()
		if err != nil {
			log.Fatalf("Cannot commit: %s", err)
		}

		err = rw.conn.Exec("DETACH DATABASE db2")
		if err != nil {
			log.Fatalf("Cannot detach (%s): %s", target, err)
		}

		os.Remove(target)
	}

	rw.conn.Close()
}

func (rw *SQLiteWriter) Close() error {
	// flush out the remainder

	if err := rw.conn.Commit(); err != nil {
		return err
	}

	if err := rw.stmt.Close(); err != nil {
		return err
	}

	if err := rw.conn.Close(); err != nil {
		return err
	}

	return nil
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

func (rw *SQLiteWriter) BeginArray(key string) {
	rw.arrayKey = key
	rw.writeToArray = true
}
func (rw *SQLiteWriter) CloseArray(key string) {
	rw.writeToArray = false

	rw.buf = append(rw.buf, strings.Join(rw.array[key], `,`))
	rw.count++

	rw.array[key] = make([]string, 0)

	rw.lastItemWasWrite = true
}
