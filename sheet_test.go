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
			{"number.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeNumeric, Value: float64(1), Index: 0}}},
			{"string.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeString, Value: "hello", Index: 0}}},
			{"multi_number.xls", 1, []xls.Cell{
				{Column: "A", Type: xls.TypeNumeric, Value: float64(1), Index: 0},
				{Column: "B", Type: xls.TypeNumeric, Value: float64(2), Index: 1},
				{Column: "C", Type: xls.TypeNumeric, Value: float64(3), Index: 2},
			}},
			{"bool.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeBoolean, Value: true, Index: 0}}},
			{"date.xls", 1, []xls.Cell{
				{Column: "A", Type: xls.TypeDate, Value: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC), Index: 0},
				{Column: "B", Type: xls.TypeDate, Value: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC), Index: 1},
				{Column: "C", Type: xls.TypeDate, Value: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC), Index: 2},
			}},
			{"pourcentage.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeNumeric, Value: float64(0.01), Index: 0}}},
			{"hour.xls", 1, []xls.Cell{{Column: "A", Type: xls.TypeDate, Value: time.Date(1900, time.January, 1, 1, 0, 0, 0, time.UTC), Index: 0}}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f, err := xls.OpenFile("testdata/" + tt.name)
				if err != nil {
					t.Fatalf("failed to open file: %v", err)
				}

				rows := make([]xls.Row, 0, tt.expectedRows)
				cells := make([]xls.Cell, 0, len(tt.expectedCells))
				for row := range f.ReadRows("Sheet 1") {
					rows = append(rows, row)

					cells = append(cells, row.Cells...)
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

	t.Run("first and last col", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name             string
			expectedFirstCol uint16
			expectedLastCol  uint16
		}{
			{"string.xls", 0, 0},
			{"multi_number.xls", 0, 2},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f, err := xls.OpenFile("testdata/" + tt.name)
				if err != nil {
					t.Fatalf("failed to open file: %v", err)
				}

				var row xls.Row
				for r := range f.ReadRows("Sheet 1") {
					row = r
					break
				}

				if row.FirstCol != tt.expectedFirstCol {
					t.Fatalf("expected FirstCol %d for %s, got %d", tt.expectedFirstCol, tt.name, row.FirstCol)
				}
				if row.LastCol != tt.expectedLastCol {
					t.Fatalf("expected LastCol %d for %s, got %d", tt.expectedLastCol, tt.name, row.LastCol)
				}
			})
		}
	})

	t.Run("malformed cell record", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
		}{
			{"invalid_number.xls"},
			{"invalid_string.xls"},
			{"invalid_bool.xls"},
			{"invalid_multi_number.xls"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f, err := xls.OpenFile("testdata/" + tt.name)
				if err != nil {
					t.Fatalf("failed to open file: %v", err)
				}

				var rowErrs []error
				for row := range f.ReadRows("Sheet 1") {
					if row.Error != nil {
						rowErrs = append(rowErrs, row.Error)
					}
				}

				if len(rowErrs) == 0 {
					t.Fatalf("expected a row error for %s, got none", tt.name)
				}
			})
		}
	})

	t.Run("unicode sheet name", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/unicode_sheet.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}

		rows := make([]xls.Row, 0, 1)
		for row := range f.ReadRows("日本語") {
			rows = append(rows, row)
		}

		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}

		cells := []xls.Cell{
			{Column: "A", Type: xls.TypeNumeric, Value: float64(1), Index: 0},
			{Column: "B", Type: xls.TypeString, Value: "日本語", Index: 1},
		}

		if len(rows[0].Cells) != len(cells) {
			t.Fatalf("expected cells %v, got %v", cells, rows[0].Cells)
		}

		for i := range cells {
			if rows[0].Cells[i] != cells[i] {
				t.Fatalf("expected cell %v for unicode sheet, got %v", cells[i], rows[0].Cells[i])
			}
		}
	})

	t.Run("big sheet", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/big.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}

		rowCount := 0
		for range f.ReadRows("Sheet 1") {
			rowCount++
		}

		if rowCount != 15000 {
			t.Fatalf("expected 15000 rows for big.xls, got %d", rowCount)
		}
	})
}
