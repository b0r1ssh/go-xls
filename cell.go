package xls

type Row struct {
	Index int
	Cells []Cell
	Error error
}

type CellType int

const (
	TypeString CellType = iota
	TypeNumeric
)

type Cell struct {
	// Column is the spreadsheet-style column label, e.g. "A", "B", "AA".
	Column string

	Type CellType
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

func addCellString(col int) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeString,
	}
}

func addCellNumeric(col int) Cell {
	return Cell{
		Column: columnLabel(col),
		Type:   TypeNumeric,
	}
}
