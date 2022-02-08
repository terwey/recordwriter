// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/productsupcom/go-sqlite-lite/sqlite3"
)

type SQLiteWriter struct {
	conn             *sqlite3.Conn
	stmt             *sqlite3.Stmt
	header           []string
	count            int
	rowCount         int
	size             int
	lastItemWasWrite bool
	array            map[string][]string
	arrayKey         string
	writeToArray     bool
	path             string
	merger           bool
	mutex            *sync.Mutex
	debugBuf         []interface{}
}

func (rw *SQLiteWriter) appendToBuf(val interface{}) {
	if DebugLog {
		rw.debugBuf = append(rw.debugBuf, val)
	}
}

func NewSQLWriter(path string, size int, merger bool) *SQLiteWriter {
	var err error
	rw := &SQLiteWriter{}
	os.Remove(path)
	rw.mutex = &sync.Mutex{}
	rw.conn, err = sqlite3.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	rw.size = size
	rw.array = make(map[string][]string)
	rw.path = path
	rw.merger = merger

	return rw
}

func (rw *SQLiteWriter) GetPath() string {
	return rw.path
}

func (rw *SQLiteWriter) cleanHeader() {
	rw.mutex.Lock()
	var buf []string
	uniq := make(map[string]int)

	// to keep compatibility with the previous header creation
	// we need to iterate once over the header and keep track if the name appeared before
	// if so, the second one to appear needs to have __2 appended, third __3 etc
	for _, name := range rw.header {
		if _, ok := uniq[strings.ToLower(name)]; ok {
			uniq[strings.ToLower(name)]++
		} else {
			uniq[strings.ToLower(name)] = 1
		}

		// sqlite doesn't care about what characters appear inside a column name as long as it's quoted
		if uniq[strings.ToLower(name)] > 1 {
			buf = append(buf, strconv.Quote(fmt.Sprintf("%s__%d", name, uniq[strings.ToLower(name)])))
		} else {
			buf = append(buf, strconv.Quote(name))
		}
	}
	rw.header = buf
	rw.mutex.Unlock()
}

func (rw *SQLiteWriter) WriteHeader(s string) {
	s = strings.ReplaceAll(s, "`", ``)
	s = strings.ReplaceAll(s, `"`, ``)
	s = strings.TrimSpace(s)
	rw.header = append(rw.header, s)
}

func (rw *SQLiteWriter) FinalizeHeader() {
	rw.cleanHeader()
	rw.mutex.Lock()
	var err error
	createQuery := fmt.Sprintf("CREATE TABLE data (%s)",
		strings.Join(rw.getCreateColumns(), `,`))
	err = rw.conn.Exec(createQuery)
	if err != nil {
		log.Fatalf("Cannot create sqlite3 header, error: %s\nQuery: %s", err, createQuery)
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

	if !rw.merger {
		rw.stmt, err = rw.conn.Prepare(rw.GetQuery())
		if err != nil {
			log.Fatalf("sqlite cannot prepare query, error: %s\nquery: %s", err, rw.GetQuery())
		}

		err = rw.conn.BeginExclusive()
		if err != nil {
			log.Fatal(err)
		}
	}

	rw.mutex.Unlock()
}

func (rw *SQLiteWriter) GetQuery() string {
	return fmt.Sprintf("INSERT INTO data (%s) VALUES (%s)",
		strings.Join(rw.header, `,`),
		strings.Join(rw.getQuestionmarks(), `,`))
}

func (rw *SQLiteWriter) Write(s string) {
	if DebugLog {
		debug("arr: %v, val: %s\n", rw.writeToArray, s)
	}
	if rw.writeToArray {
		rw.array[rw.arrayKey] = append(rw.array[rw.arrayKey], s)
		return
	}

	if len(rw.header) > rw.count {
		debug("Writing to %s = %s", rw.header[rw.count], s)
	} else {
		if Overflow {
			debug("Overflowing %d = %s", rw.count, s)
		} else {
			return
		}
	}

	if err := rw.stmt.BindString(s, rw.count); err != nil {
		fmt.Fprintf(os.Stderr, "header (%d): \n%v\n\n", len(rw.header), rw.header)
		fmt.Fprintf(os.Stderr, "writing: %s to %d", s, rw.count)
		rw.handleError(err)
	}
	rw.appendToBuf(s)
	rw.count++
	rw.lastItemWasWrite = true
}

// WriteByte uses an unsafe.Pointer method to return the incoming []byte as a string
// beware: could eat your dog
func (rw *SQLiteWriter) WriteByte(s []byte) {

	if len(rw.header) > rw.count {
		debug("Writing to %s = %s", rw.header[rw.count], s)
	} else {
		if Overflow {
			debug("Overflowing %d = %s", rw.count, s)
		} else {
			return
		}
	}

	if err := rw.stmt.BindString(ByteSlice2String(s), rw.count); err != nil {
		rw.handleError(err)
		// log.Printf("SQLiteWriter cannot WriteByte (count: %d): %s\nError: %s", rw.count, s, err)
		// panic(fmt.Sprintf("SQLiteWriter cannot WriteByte (count: %d): %s\nError: %s", rw.count, s, err))
	}
	rw.appendToBuf(s)
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
		if err := rw.stmt.BindString(s, rw.count); err != nil {
			log.Fatal(err)
		}
	}

	rw.count++
	rw.lastItemWasWrite = true
}

func (rw *SQLiteWriter) Next() {
	if DebugLog {
		debug("Next")
	}
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) WriteNull() {
	if DebugLog {
		debug("WriteNull")
	}
	if err := rw.stmt.BindEmptyString(rw.count); err != nil {
		if len(rw.header) > rw.count {
			debug("Skipping for %s", rw.header[rw.count])
		} else {
			if Overflow {
				debug("Overflowing skip %d", rw.count)
			} else {
				return
			}
		}
		rw.handleError(err)
	}
	rw.appendToBuf(nil)
	rw.count++
	rw.lastItemWasWrite = true
}

func (rw *SQLiteWriter) Skip() {
	if DebugLog {
		if len(rw.header) > rw.count {
			debug("Skip called for %s", rw.header[rw.count])
		} else {
			debug("Skip called for overflowing skip %d", rw.count)
		}
	}
	if !rw.lastItemWasWrite {
		if len(rw.header) > rw.count {
			debug("Skipping REALLY for %s", rw.header[rw.count])
		} else {
			if Overflow {
				debug("Overflowing REALLY skip %d", rw.count)
			} else {
				return
			}
		}

		if err := rw.stmt.BindNull(rw.count); err != nil {
			rw.handleError(err)
		}

		// originally this wrote away a nil but the existing implementation writes an empty string
		// if err := rw.stmt.BindEmptyString(rw.count); err != nil {
		// 	rw.handleError(err)
		// 	// log.Fatal(err)
		// }
		rw.appendToBuf(nil)
		rw.count++
	}
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) EOR(columns int) {
	var err error
	rw.mutex.Lock()

	if rw.count < columns {
		for rw.count < columns {
			if err := rw.stmt.BindEmptyString(rw.count); err != nil {
				log.Fatal(err)
			}
			rw.count++
		}
	}

	err = rw.stmt.ExecPrebound()

	if err != nil {
		rw.handleError(err)
	}

	if rw.rowCount > rw.size {
		if err = rw.conn.Commit(); err != nil {
			log.Fatal(err)
		}
		err = rw.conn.BeginExclusive()
		if err != nil {
			log.Fatal(err)
		}

		rw.rowCount = 0
	}

	if DebugLog {
		rw.debugBuf = make([]interface{}, 0)
	}
	rw.count = 0
	rw.rowCount++
	rw.mutex.Unlock()
}

func (rw *SQLiteWriter) handleError(err error) {
	// header := rw.getCreateColumns()
	print("\n----------")
	print("Failed importing to SQLite")
	print("header (len: %d): \n----------\n%v", len(rw.header), rw.header)
	print("%s", err)
	print("----------")
	if len(rw.debugBuf) != 0 {
		print("Data in buffer:")
		for i, s := range rw.debugBuf {
			if len(rw.header) > i {
				if data, ok := s.([]uint8); ok {
					print("%s (%d): %s", rw.header[i], i, data)
				} else {
					print("%s (%d): %v", rw.header[i], i, s)
				}
			} else {
				if data, ok := s.([]uint8); ok {
					print("%d: %s", i, data)
				} else {
					print("%d: %v", i, s)
				}
			}
		}
		print("----------")
	}
	print("Ending")
	panic("Failed to process SQLite writer")
}

func (rw *SQLiteWriter) Merge(ready <-chan string, wg *sync.WaitGroup, duration chan time.Duration) {
	defer wg.Done()
	started := false
	var start time.Time
	for target := range ready {
		if !started {
			started = true
			start = time.Now()
		}

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
			// we wait a few ms and then try again, if it fails again just exit
			time.Sleep(5 * time.Millisecond)
			err = rw.conn.Commit()
			if err != nil {
				log.Fatalf("Cannot commit for target (%s): %s", target, err)
			}
		}

		err = rw.conn.Exec("DETACH DATABASE db2")
		if err != nil {
			log.Fatalf("Cannot detach (%s): %s", target, err)
		}

		os.Remove(target)
	}

	duration <- time.Since(start)
	rw.conn.Close()
}

func (rw *SQLiteWriter) Close() error {
	rw.mutex.Lock()

	if err := rw.conn.Commit(); err != nil {
		return err
	}

	if err := rw.stmt.Close(); err != nil {
		return err
	}

	if err := rw.conn.Close(); err != nil {
		return err
	}

	rw.mutex.Unlock()

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
	if key == `` {
		panic("BeginArray Key cannot be empty")
	}
	fmt.Fprintf(os.Stderr, "BeginArray %s\n", key)
	rw.arrayKey = key
	rw.writeToArray = true
}
func (rw *SQLiteWriter) CloseArray(key string) {
	if key == `` {
		panic("CloseArray Key cannot be empty")
	}
	if DebugLog {
		debug("CloseArray (%s): %s\n", key, strings.Join(rw.array[key], `,`))
	}
	rw.writeToArray = false

	if err := rw.stmt.BindString(strings.Join(rw.array[key], `,`), rw.count); err != nil {
		log.Fatal(err)
	}

	rw.count++

	rw.array[key] = make([]string, 0)

	rw.lastItemWasWrite = true
}
