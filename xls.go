// Package xls reads legacy Microsoft Excel .xls (BIFF8) workbooks. It
// exposes the sheet names of a workbook and streams each sheet's rows
// without loading the whole sheet into memory.
package xls

import (
	"errors"
	"io"
	"os"

	"github.com/b0r1ssh/go-xls/internal/cfb"
)

type streamReader interface {
	io.Reader
	io.Seeker
}

// Sheet is a worksheet within a workbook.
type Sheet struct {
	// Name is the worksheet's name as shown in Excel.
	name string

	offset uint32
}

// File is an opened .xls workbook. Call Close when done with it.
type File struct {
	closer io.Closer

	stream streamReader

	sheets []Sheet

	// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/3f52609d-816f-44a7-aad1-e0fe2abccebd
	sst []string
}

// OpenFile opens the .xls workbook at path. The returned File must be
// closed with Close.
func OpenFile(path string) (*File, error) {
	o, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	f, err := OpenReader(o)
	if err != nil {
		o.Close()
		return nil, err
	}

	f.closer = o
	return f, nil
}

// OpenReader opens a .xls workbook from r. Unlike OpenFile, the caller
// retains ownership of r and Close will not close it.
func OpenReader(r io.ReaderAt) (*File, error) {
	container, err := cfb.Open(r)
	if err != nil {
		return nil, err
	}

	stream, err := container.Stream("Workbook")
	if err != nil {
		return nil, err
	}

	f := &File{
		stream: stream,
	}

	if err := f.compute(); err != nil {
		return nil, err
	}

	return f, nil
}

// Close releases resources associated with the File. If the File was
// obtained with OpenReader, Close is a no-op.
func (f *File) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}

// SheetNames returns the names of the workbook's visible worksheets, in
// the order they appear in Excel.
func (f *File) SheetNames() []string {
	names := make([]string, len(f.sheets))
	for i, sheet := range f.sheets {
		names[i] = sheet.name
	}
	return names
}

func (f *File) compute() error {
	rr := &recordReader{r: f.stream}

	var sheets []Sheet

loop:
	for {
		rec, err := rr.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break loop
			}

			return err
		}

		switch rec.recType {
		case recEOF:
			break loop
		case recBOUNDSHEET:
			offset, sheetType, name, err := rec.parseBOUNDSHEET()
			if err != nil {
				return err
			}

			// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/b9ec509a-235d-424e-871d-f8e721106501
			// Only include visible worksheets (sheetType == 0x00)
			if sheetType == 0x00 {
				sheets = append(sheets, Sheet{
					name:   name,
					offset: offset,
				})
			}
		case recSST:
			sst, err := rec.parseSST()
			if err != nil {
				return err
			}

			f.sst = sst
		}
	}

	f.sheets = sheets

	return nil
}
