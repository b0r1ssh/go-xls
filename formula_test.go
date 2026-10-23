package xls_test

import (
	"testing"

	"github.com/b0r1ssh/go-xls"
)

func TestXLSReadFormulaResults(t *testing.T) {
	t.Parallel()

	f, err := xls.OpenFile("testdata/formula.xls")
	if err != nil {
		t.Fatalf("failed to open formula.xls: %v", err)
	}

	want := []xls.Cell{
		{Column: "A", Type: xls.TypeNumeric, Value: float64(3), Index: 0},
		{Column: "A", Type: xls.TypeBoolean, Value: true, Index: 0},
		{Column: "A", Type: xls.TypeError, Value: "#DIV/0!", Index: 0},
		{Column: "A", Type: xls.TypeString, Value: "text", Index: 0},
		{Column: "A", Type: xls.TypeNumeric, Value: float64(4), Index: 0},
	}

	rows := 0
	for row := range f.ReadRows("Sheet 1") {
		if row.Error != nil {
			t.Fatalf("row %d error: %v", row.Index, row.Error)
		}

		if rows >= len(want) {
			t.Fatalf("unexpected extra row %d", row.Index)
		}

		if row.Index != rows {
			t.Fatalf("row index = %d, want %d", row.Index, rows)
		}

		var result xls.Cell
		for _, cell := range row.Cells {
			if cell.Index == 0 {
				result = cell
				break
			}
		}

		if expected := want[rows]; result != expected {
			t.Errorf("row %d result = %#v, want %#v", row.Index, result, expected)
		}

		rows++
	}

	if rows != len(want) {
		t.Fatalf("read %d rows, want %d", rows, len(want))
	}
}
