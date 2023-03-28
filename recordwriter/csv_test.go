package recordwriter

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/terwey/csv2sqlite/characters"
	"github.com/terwey/csv2sqlite/characters/knownconfigs"
)

type csvTest struct {
	name       string
	s          string
	want       string
	config     characters.Config
	delimiter  characters.Character
	enclosure  characters.Character
	quoteAll   bool
	quoteEmpty bool
}

func getTests() []csvTest {
	return []csvTest{
		{
			name:   "needs enclosure and enclosure needs escape",
			s:      `"`,
			want:   `""""`,
			config: knownconfigs.CSV(),
		},
		{
			name: "no enclosure",
			s:    `foo`,
			want: `foo`,
		},
		{
			name:     "no enclosure required, quote all",
			s:        `foo`,
			want:     `"foo"`,
			quoteAll: true,
		},
		{
			name:   "needs enclosure",
			s:      `foo, foo`,
			want:   `"foo, foo"`,
			config: knownconfigs.CSV(),
		},
		{
			name: "needs enclosure newline",
			s: `foo
 foo`,
			want: `"foo
 foo"`,
			config: knownconfigs.CSV(),
		},
		{
			name:       "quote empty false",
			s:          ``,
			want:       ``,
			config:     knownconfigs.CSV(),
			quoteEmpty: false,
		},
		{
			name:       "quote empty false, quoteall true (should not quote)",
			s:          ``,
			want:       ``,
			config:     knownconfigs.CSV(),
			quoteEmpty: false,
			quoteAll:   true,
		},
		{
			name:       "quote empty true",
			s:          ``,
			want:       `""`,
			config:     knownconfigs.CSV(),
			quoteEmpty: true,
		},
	}
}

func TestCSVWriter_WriteByte(t *testing.T) {
	for _, tt := range getTests() {
		t.Run(tt.name, func(t *testing.T) {
			b := bytes.NewBuffer([]byte{})
			c := knownconfigs.CSV()
			if len(tt.delimiter) != 0 {
				c.Delimiter = tt.delimiter
			}
			if len(tt.enclosure) != 0 {
				c.Enclosure = tt.enclosure
			}
			csv := NewCSVWriter(c, b, "")

			csv.WriteByte([]byte(tt.s))
			csv.Close()

			got := b.String()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("TestCSVWriter_WriteByte() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCSVWriter_Write(t *testing.T) {
	for _, tt := range getTests() {
		t.Run(tt.name, func(t *testing.T) {
			b := bytes.NewBuffer([]byte{})
			c := knownconfigs.CSV()
			if len(tt.delimiter) != 0 {
				c.Delimiter = tt.delimiter
			}
			if len(tt.enclosure) != 0 {
				c.Enclosure = tt.enclosure
			}
			csv := NewCSVWriter(c, b, "")

			csv.Write(tt.s)
			csv.Close()

			got := b.String()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("TestCSVWriter_Write() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
