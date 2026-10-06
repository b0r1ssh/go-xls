package xls

import (
	"encoding/binary"
	"fmt"
	"io"
)

func (f *File) ReadRows(sheetName string) chan Row {
	ch := make(chan Row, 32)
	sheet, ok := f.sheetByName(sheetName)
	if !ok {
		close(ch)
		return ch
	}
	go f.readSheetRows(sheet, ch)
	return ch
}

func (f *File) sheetByName(name string) (Sheet, bool) {
	for _, sheet := range f.sheets {
		if sheet.name == name {
			return sheet, true
		}
	}
	return Sheet{}, false
}

func (f *File) readSheetRows(sheet Sheet, ch chan<- Row) {
	defer close(ch)

	stream := f.stream
	if _, err := stream.Seek(int64(sheet.offset), io.SeekStart); err != nil {
		ch <- Row{Error: err}
		return
	}

	rr := &recordReader{r: stream}

	cur := Row{Index: -1}

	rowWidth := 0
	flush := func() {
		if cur.Index >= 0 {
			if len(cur.Cells) > rowWidth {
				rowWidth = len(cur.Cells)
			}
			ch <- cur
		}
		cur = Row{Index: -1}
	}

	fail := func(err error) {
		flush()
		ch <- Row{Error: err}
	}

	add := func(rw int, c Cell) {
		if cur.Index != rw {
			flush()
			cur.Index = rw
			if rowWidth > 0 {
				cur.Cells = make([]Cell, 0, rowWidth)
			}
		}
		cur.Cells = append(cur.Cells, c)
	}

	for {
		rec, err := rr.next()
		if err != nil {
			if err != io.EOF {
				fail(err)
			} else {
				flush()
			}
			return
		}

		switch rec.recType {
		case recEOF:
			flush()
			return

		case recROW:
			if len(rec.data) < 2 {
				continue
			}

			rw := binary.LittleEndian.Uint16(rec.data[0:2])
			if cur.Index != int(rw) {
				flush()
				cur.Index = int(rw)
			}
		case recRK:
			rw, col, value, err := rec.parseRK()
			if err != nil {
				fail(err)
				continue
			}

			add(rw, addCellNumeric(col, value))
		case recMULRK:
			rw, cols, values, err := rec.parseMULRK()
			if err != nil {
				fail(err)
				continue
			}

			for i := range cols {
				add(rw, addCellNumeric(cols[i], values[i]))
			}
		case recLABELSST:
			if len(rec.data) < 10 {
				continue
			}

			rw := int(binary.LittleEndian.Uint16(rec.data[0:2]))
			col := int(binary.LittleEndian.Uint16(rec.data[2:4]))

			// TODO: Use SST
			// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/3f52609d-816f-44a7-aad1-e0fe2abccebd
			add(rw, addCellString(col, ""))
		case recBOOLERR:
			rw, col, value, err := rec.parseBOOLERR()
			if err != nil {
				fail(err)
				continue
			}

			add(rw, addCellBoolean(col, value))
		default:
			fmt.Printf("unknown record type: %#04x\n", rec.recType)

			continue
		}

	}
}
