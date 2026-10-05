package xls

import (
	"errors"
	"io"
	"os"

	"github.com/b0r1sh/go-xls/internal/cfb"
)

type streamReader interface {
	io.Reader
	io.Seeker
}

type Sheet struct {
	// Name is the worksheet's name as shown in Excel.
	Name string

	offset uint32
}

type File struct {
	closer io.Closer

	stream streamReader

	sheets []Sheet
}

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

func (f *File) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}

func (f *File) SheetNames() []string {
	names := make([]string, len(f.sheets))
	for i, sheet := range f.sheets {
		names[i] = sheet.Name
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
			offset, sheetType, name, err := parseBOUNDSHEET(rec)
			if err != nil {
				return err
			}

			// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/b9ec509a-235d-424e-871d-f8e721106501
			// Only include visible worksheets (sheetType == 0x00)
			if sheetType == 0x00 {
				sheets = append(sheets, Sheet{
					Name:   name,
					offset: offset,
				})
			}
		}
	}

	f.sheets = sheets

	return nil
}
