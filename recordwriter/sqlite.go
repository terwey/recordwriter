// SPDX-License-Identifier: MIT
// Copyright (c) 2020 Yorick Terweijden

package recordwriter

import (
	"fmt"
	"hash/fnv"
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
	rowid            bool

	processedHeaderSet bool
}

func NewSQLWriter(path string, size int, merger bool, rowid bool) *SQLiteWriter {
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
	rw.rowid = rowid

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
	if rw.processedHeaderSet {
		return
	}
	s = strings.ReplaceAll(s, "`", ``)
	s = strings.ReplaceAll(s, `"`, ``)
	s = strings.TrimSpace(s)
	rw.header = append(rw.header, s)
}

// SetProcessedHeader expects a fully finalized Header including quotations for SQLite
func (rw *SQLiteWriter) SetProcessedHeader(header []string) {
	rw.header = header
	rw.processedHeaderSet = true
}

func (rw *SQLiteWriter) FinalizeHeader() []string {
	if !rw.processedHeaderSet {
		rw.cleanHeader()
	}
	rw.mutex.Lock()
	var err error
	createQuery := fmt.Sprintf("CREATE TABLE data (%s)",
		strings.Join(rw.getCreateColumns(), `,`))
	err = rw.conn.Exec(createQuery)
	if err != nil {
		log.Fatalf("Cannot create sqlite3 header, error: %s\nQuery: %s", err, createQuery)
	}

	if err = rw.conn.Exec("PRAGMA synchronous = 0"); err != nil {
		log.Fatal(err)
	}

	if err = rw.conn.Exec("PRAGMA mmap_size=468435456"); err != nil {
		log.Fatal(err)
	}

	// if err = rw.conn.Exec("PRAGMA cache_size=-1024000000"); err != nil {
	// 	log.Fatal(err)
	// }

	if err = rw.conn.Exec("PRAGMA locking_mode = EXCLUSIVE"); err != nil {
		log.Fatal(err)
	}

	if rw.merger {
		if err = rw.conn.Exec("PRAGMA journal_mode = WAL2"); err != nil {
			log.Fatal(err)
		}
	}

	if !rw.merger {
		if err = rw.conn.Exec("PRAGMA journal_mode = OFF"); err != nil {
			log.Fatal(err)
		}

		if err = rw.conn.Exec("PRAGMA temp_store = MEMORY;"); err != nil {
			log.Fatal(err)
		}

		rw.stmt, err = rw.conn.PreparePersistent(rw.GetQuery())
		if err != nil {
			log.Fatalf("sqlite cannot prepare query, error: %s\nquery: %s", err, rw.GetQuery())
		}

		err = rw.conn.BeginExclusive()
		if err != nil {
			log.Fatal(err)
		}
	}

	rw.mutex.Unlock()

	return rw.header
}

func (rw *SQLiteWriter) GetQuery() string {
	return fmt.Sprintf("INSERT INTO data (%s) VALUES (%s)",
		strings.Join(rw.header, `,`),
		strings.Join(rw.getQuestionmarks(), `,`))
}

// Multibyte is a highly optimized call, it also internally already calls ExecPrebound
// Must be used in conjunction with EORMulti()
func (rw *SQLiteWriter) Multibyte(out []byte, pos []uint32, columns int) {
	if err := rw.stmt.BindMultibytes(out, pos, len(pos), columns); err != nil {
		rw.handleError(err)
	}

	rw.mutex.Lock()
	rw.count = len(pos)
	rw.mutex.Unlock()
}

func (rw *SQLiteWriter) Write(s string) {
	debug("arr: %v, val: %s\n", rw.writeToArray, s)
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

	if err := rw.stmt.BindBytes(s, rw.count); err != nil {
		rw.handleError(err)
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
	debug("Next")
	rw.lastItemWasWrite = false
}

func (rw *SQLiteWriter) WriteNull() {
	debug("WriteNull")
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
	if len(rw.header) > rw.count {
		debug("Skip called for %s", rw.header[rw.count])
	} else {
		debug("Skip called for overflowing skip %d", rw.count)
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

	rw.debugBuf = make([]interface{}, 0)
	rw.count = 0
	rw.rowCount++
	rw.mutex.Unlock()
}

func (rw *SQLiteWriter) EORMulti() {
	var err error
	rw.mutex.Lock()

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

	rw.debugBuf = make([]interface{}, 0)
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

	var columnNames []string
	started := false
	var start time.Time
	var mergeWg sync.WaitGroup
	for target := range ready {
		if !started {
			started = true
			start = time.Now()
		}

		mergeWg.Add(1)
		go func(src string, target string, w *sync.WaitGroup) {
			defer w.Done()
			conn, err := sqlite3.Open(src)
			if err != nil {
				log.Fatalf("Cannot open (%s): %s", src, err)
			}

			// always return true
			conn.BusyFunc(func(count int) (retry bool) {
				return true
			})
			if err = conn.Exec("PRAGMA journal_mode = WAL2"); err != nil {
				log.Fatal(err)
			}
			if err = conn.Exec("PRAGMA synchronous = 0"); err != nil {
				log.Fatal(err)
			}
			if err = conn.Exec("PRAGMA mmap_size=468435456"); err != nil {
				log.Fatal(err)
			}

			if rw.rowid && len(columnNames) == 0 {
				columnNames, err = ColumnsForDB("data", conn)
				if err != nil {
					log.Fatalf("Cannot get ColumnsForDB: %s", err)
				}
			}

			h := fnv.New32a()
			h.Write([]byte(target))

			err = conn.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS db%d", target, h.Sum32()))
			if err != nil {
				log.Fatalf("Cannot attach (%s): %s", target, err)
			}

			err = conn.BeginExclusive()
			if err != nil {
				log.Fatalf("Cannot begin: %s", err)
			}

			if rw.rowid {
				err = conn.Exec(fmt.Sprintf("INSERT INTO main.data (rowid, %s) SELECT rowid, %s FROM db%d.data", strings.Join(columnNames, ","), strings.Join(columnNames, ","), h.Sum32()))
				if err != nil {
					log.Fatalf("Merger: Cannot insert (%s): %s", target, err)
				}
			} else {
				err = conn.Exec(fmt.Sprintf("INSERT INTO main.data SELECT * FROM db%d.data", h.Sum32()))
				if err != nil {
					log.Fatalf("Merger: Cannot insert (%s): %s", target, err)
				}
			}

			err = conn.Commit()
			if err != nil {
				// we wait a few ms and then try again, if it fails again just exit
				time.Sleep(5 * time.Millisecond)
				log.Printf("commit after sleep: %s", target)
				err = conn.Commit()
				if err != nil {
					log.Fatalf("Cannot commit for target (%s): %s", target, err)
				}
			}

			err = conn.Exec(fmt.Sprintf("DETACH DATABASE db%d", h.Sum32()))
			if err != nil {
				log.Fatalf("Cannot detach (%s): %s", target, err)
			}

			os.Remove(target)
			err = conn.Close()
		}(rw.path, target, &mergeWg)
	}

	mergeWg.Wait()

	duration <- time.Since(start)
	rw.conn.Close()
}

func ColumnsForDB(name string, conn *sqlite3.Conn) ([]string, error) {
	stmt, err := conn.Prepare("select name from pragma_table_info('" + name + "') order by name;")
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	var names []string

	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return nil, err
		}
		if !hasRow {
			break
		}

		var name string
		err = stmt.Scan(&name)
		if err != nil {
			return nil, err
		}

		// we don't want to explicitly select for rowid
		if name == "rowid" {
			continue
		}

		names = append(names, `"`+name+`"`)
	}

	return names, nil
}

func (rw *SQLiteWriter) Close() error {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

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
		if v == `"rowid"` {
			continue
		}
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
	debug("CloseArray (%s): %s\n", key, strings.Join(rw.array[key], `,`))
	rw.writeToArray = false

	if err := rw.stmt.BindString(strings.Join(rw.array[key], `,`), rw.count); err != nil {
		log.Fatal(err)
	}

	rw.count++

	rw.array[key] = make([]string, 0)

	rw.lastItemWasWrite = true
}
