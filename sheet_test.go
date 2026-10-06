package xls_test

import (
	"testing"
	"time"

	"github.com/b0r1ssh/go-xls"
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
			name          string
			expectedRows  int
			expectedCells []xls.Cell
		}{
			{"number.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeNumeric, Value: float64(1)}}},
			{"string.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeString, Value: "hello"}}},
			{"multi_number.xls", 1, []xls.Cell{
				{Column: "A", Type: xls.TypeNumeric, Value: float64(1)},
				{Column: "B", Type: xls.TypeNumeric, Value: float64(2)},
				{Column: "C", Type: xls.TypeNumeric, Value: float64(3)},
			}},
			{"bool.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeBoolean, Value: true}}},
			{"date.xls", 1, []xls.Cell{
				{Column: "A", Type: xls.TypeDate, Value: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC)},
				{Column: "B", Type: xls.TypeDate, Value: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC)},
				{Column: "C", Type: xls.TypeDate, Value: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC)},
			}},
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

				rows := make([]xls.Row, 0, tt.expectedRows)
				cells := make([]xls.Cell, 0, len(tt.expectedCells))
				for row := range f.ReadRows("Sheet 1") {
					rows = append(rows, row)

					for _, cell := range row.Cells {
						cells = append(cells, cell)
					}
				}

				if len(rows) != tt.expectedRows {
					t.Fatalf("expected %d rows for %s, got %d", tt.expectedRows, tt.name, len(rows))
				}

				for i := range tt.expectedRows {
					if rows[i].Index != i {
						t.Fatalf("expected row index %d for %s, got %d", i, tt.name, rows[i].Index)
					}
				}

				if len(cells) != len(tt.expectedCells) {
					t.Fatalf("expected %d cols for %s, got %d", len(tt.expectedCells), tt.name, len(cells))
				}

				for i := range tt.expectedCells {
					if cells[i] != tt.expectedCells[i] {
						t.Fatalf("expected cell %v for %s, got %v", tt.expectedCells[i], tt.name, cells[i])
					}
				}
			})
		}
	})

	t.Run("big sheet", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/all.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Fatalf("failed to close file")
			}
		})

		rowCount := 0
		for range f.ReadRows("Sheet 1") {
			rowCount++
		}

		if rowCount != 15000 {
			t.Fatalf("expected 15000 rows for all.xls, got %d", rowCount)
		}
	})
}
