package xls_test

import (
	"testing"

	"github.com/b0r1sh/go-xls"
)

func TestXLSReadRows(t *testing.T) {
	t.Parallel()

	t.Run("valid sheet", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/valid.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Fatalf("failed to close file")
			}
		})

		i := 0
		for range f.ReadRows("Sheet 1") {
			i++
		}

		if i == 0 {
			t.Fatalf("expected rows for valid sheet, got 0")
		}
	})

	t.Run("invalid sheet", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/valid.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Fatalf("failed to close file")
			}
		})

		i := 0
		for range f.ReadRows("Sheet 2") {
			i++
		}

		if i != 0 {
			t.Fatalf("expected 0 rows for invalid sheet, got %d", i)
		}
	})

	t.Run("basic cell type", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			expectedRows int
			expectedCols int
		}{
			{"number.xls", 1, 1},
			{"string.xls", 1, 1},
			{"hour.xls", 1, 1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f, err := xls.OpenFile("testdata/" + tt.name)
				if err != nil {
					t.Fatalf("failed to open file: %v", err)
				}
				t.Cleanup(func() {
					if err := f.Close(); err != nil {
						t.Fatalf("failed to close file")
					}
				})

				i := 0
				j := 0
				for row := range f.ReadRows("Sheet 1") {
					i++

					j = len(row.Cells)
				}

				if i != tt.expectedRows {
					t.Fatalf("expected %d rows for %s, got %d", tt.expectedRows, tt.name, i)
				}

				if j != tt.expectedCols {
					t.Fatalf("expected %d cols for %s, got %d", tt.expectedCols, tt.name, j)
				}
			})
		}
	})
}
