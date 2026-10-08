package xls

import "time"

// Row is one row of cells read from a sheet. If Error is non-nil, the
// other fields are not meaningful.
type Row struct {
	// Index is the 0-based index of this row within the sheet.
	Index int

	// Cells holds the individual cells in this row.
	Cells []Cell

	// Error holds any error encountered while reading this row.
	Error error

	// FirstCol and LastCol are the 0-based range of columns that carry
	// data in this row, as recorded by Excel's ROW record.
	FirstCol uint16
	LastCol  uint16
}

// CellType identifies the Go type stored in a Cell's Value.
type CellType uint8

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

	// Value holds the actual value of the cell. Its type is indicated by Type.
	Value any

	// Index is the 0-based index of this cell within its row.
	Index int

	// Type indicates the type of value stored in this cell.
	Type CellType
}

var columnNames = func() [256]string {
	var names [256]string
	for i := range names {
		names[i] = makeColumnName(i)
	}
	return names
}()

// columnLabel converts a 0-based column index into its spreadsheet-style
// label (0 -> "A", 25 -> "Z", 26 -> "AA", ...).
func columnLabel(col int) string {
	if col >= 0 && col < len(columnNames) {
		return columnNames[col]
	}
	return makeColumnName(col)
}

func makeColumnName(col int) string {
	var buf [8]byte
	i := len(buf)
	col++
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
		Index:  col,
	}
}

func addCellNumeric(col int, value float64) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeNumeric,
		Value:  value,
		Index:  col,
	}
}

func addCellBoolean(col int, value bool) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeBoolean,
		Value:  value,
		Index:  col,
	}
}

func addCellDate(col int, value time.Time) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeDate,
		Value:  value,
		Index:  col,
	}
}
