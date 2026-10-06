package xls

import "time"

// Row is one row of cells read from a sheet. If Error is non-nil, the
// other fields are not meaningful.
type Row struct {
	Index int
	Cells []Cell
	Error error
}

// CellType identifies the Go type stored in a Cell's Value.
type CellType int

const (
	// TypeString means Value is a string.
	TypeString CellType = iota
	// TypeNumeric means Value is a float64.
	TypeNumeric
	// TypeBoolean means Value is a bool.
	TypeBoolean
	// TypeDate means Value is a time.Time.
	TypeDate
)

// Cell is a single spreadsheet cell.
type Cell struct {
	// Column is the spreadsheet-style column label, e.g. "A", "B", "AA".
	Column string

	Type CellType

	Value any
}

// columnLabel converts a 0-based column index into its spreadsheet-style
// label (0 -> "A", 25 -> "Z", 26 -> "AA", ...).
func columnLabel(col int) string {
	col++
	var buf [8]byte
	i := len(buf)
	for col > 0 {
		col--
		i--
		buf[i] = byte('A' + col%26)
		col /= 26
	}
	return string(buf[i:])
}

func addCellString(col int, value string) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeString,
		Value:  value,
	}
}

func addCellNumeric(col int, value float64) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeNumeric,
		Value:  value,
	}
}

func addCellBoolean(col int, value bool) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeBoolean,
		Value:  value,
	}
}

func addCellDate(col int, value time.Time) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeDate,
		Value:  value,
	}
}
