package recordwriter

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/productsupcom/go-sqlite-lite/sqlite3"
)

// TestSQLiteWriter_storedValues expects WriteByte to store TEXT and Skip to store
// an empty string, matching what Multibyte stores.
func TestSQLiteWriter_storedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.sqlite")

	rw := NewSQLWriter(path, 1000, false, false)
	for _, h := range []string{"a", "b", "c"} {
		rw.WriteHeader(h)
	}
	rw.FinalizeHeader()

	rw.WriteByte([]byte("1"))
	rw.Skip()
	rw.WriteByte([]byte("3"))
	rw.EOR(3)

	rw.WriteByte([]byte("4"))
	rw.EOR(3)

	if err := rw.Close(); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"1", "", "3", "text", "text", "text"},
		{"4", "", "", "text", "text", "text"},
	}
	got := readRows(t, path, "SELECT a, b, c, typeof(a), typeof(b), typeof(c) FROM data ORDER BY a")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("stored rows mismatch (-want +got):\n%s", diff)
	}
}

func readRows(t *testing.T, path, query string) [][]string {
	t.Helper()

	conn, err := sqlite3.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	stmt, err := conn.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var rows [][]string
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			t.Fatal(err)
		}
		if !hasRow {
			return rows
		}

		row := make([]string, stmt.ColumnCount())
		for i := range row {
			row[i], _, err = stmt.ColumnText(i)
			if err != nil {
				t.Fatal(err)
			}
		}
		rows = append(rows, row)
	}
}
