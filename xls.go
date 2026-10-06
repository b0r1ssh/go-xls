// Package xls reads legacy Microsoft Excel .xls (BIFF8) workbooks. It
// exposes the sheet names of a workbook and streams each sheet's rows
// without loading the whole sheet into memory.
package xls

import (
	"bytes"
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
	stream streamReader

	sheets []Sheet

	// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/3f52609d-816f-44a7-aad1-e0fe2abccebd
	sst []string

	// dateSystem1904 indicates whether the workbook uses the 1904 date system
	// (epoch 1904-01-01) instead of the default 1900 system.
	// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/4a5e900a-0eb0-4355-8fc1-81aab8f46e8b
	dateSystem1904 bool

	// dateXFECache caches isDateXFE's result per XF record index; computed
	// once after parsing since formats/xfFormats don't change afterward.
	dateXFECache []bool
}

// OpenFile opens the .xls workbook at path. The returned File must be
// closed with Close.
func OpenFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// read the whole file up front so the CFB reader serves sectors from
	// memory instead of issuing a syscall per sector.
	return OpenReader(bytes.NewReader(data))
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

	var (
		sheets []Sheet

		// formats maps a custom number format index (ifmt) to its
		// format code, as defined by FORMAT records.
		// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/300280fd-e4fe-4675-a924-4d383af48d3b
		formats map[uint16]string

		// xfFormats holds the number format index (ifmt) of each XF record,
		// in the order the XF records appear in the stream.
		// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/993d15c4-ec04-43e9-ba36-594dfb336c6d
		xfFormats []uint16
	)

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
		case recFORMAT:
			ifmt, code, err := rec.parseFORMAT()
			if err != nil {
				return err
			}

			if formats == nil {
				formats = make(map[uint16]string)
			}

			formats[ifmt] = code
		case recXF:
			ifmt, err := rec.parseXF()
			if err != nil {
				return err
			}

			xfFormats = append(xfFormats, ifmt)
		case recDATEMODE:
			is1904, err := rec.parseDATEMODE()
			if err != nil {
				return err
			}

			f.dateSystem1904 = is1904
		}
	}

	f.sheets = sheets

	dateXFECache := make([]bool, len(xfFormats))
	for i, ifmt := range xfFormats {
		dateXFECache[i] = isDateXFECode(formats, ifmt)
	}
	f.dateXFECache = dateXFECache

	return nil
}
