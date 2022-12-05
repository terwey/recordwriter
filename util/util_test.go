package util

import (
	"reflect"
	"testing"
)

func TestCleanHeader(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		want   []string
	}{
		{
			name:   "two columns no dupes or control",
			header: []string{"col1", "col2"},
			want:   []string{"\"col1\"", "\"col2\""},
		},
		{
			name:   "two columns no dupes with control",
			header: []string{"\u0095col1", "col2"},
			want:   []string{"\"col1\"", "\"col2\""},
		},
		{
			name:   "two columns with dupes no control",
			header: []string{"col1", "col2", "col1"},
			want:   []string{"\"col1\"", "\"col2\"", "\"col1__2\""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanHeader(tt.header); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CleanHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}
