package xls

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

// BIFF8 record type identifiers used by this package.
// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/6fba0383-0d7a-4c7a-afe9-642ff70cbd36
const (
	recEOF        = 0x000A
	recROW        = 0x0208
	recBOUNDSHEET = 0x0085
	recRK         = 0x027E
	recLABELSST   = 0x00FD
)

type biffRecord struct {
	recType uint16
	data    []byte
}

type recordReader struct {
	r   io.Reader
	buf []byte
}

func (rr *recordReader) next() (biffRecord, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(rr.r, hdr[:]); err != nil {
		return biffRecord{}, fmt.Errorf("failed to read BIFF record header: %w", err)
	}

	recType := binary.LittleEndian.Uint16(hdr[0:2])
	length := int(binary.LittleEndian.Uint16(hdr[2:4]))
	if cap(rr.buf) < length {
		rr.buf = make([]byte, length)
	}

	data := rr.buf[:length]
	if length > 0 {
		if _, err := io.ReadFull(rr.r, data); err != nil {
			return biffRecord{}, fmt.Errorf("failed to read BIFF record body: %w", err)
		}
	}
	return biffRecord{recType: recType, data: data}, nil
}

func parseBOUNDSHEET(rec biffRecord) (offset uint32, sheetType byte, name string, err error) {
	if len(rec.data) < 8 {
		return 0, 0, "", errors.New("malformed BOUNDSHEET record")
	}

	offset = binary.LittleEndian.Uint32(rec.data[0:4])
	sheetType = rec.data[5]
	cch := int(rec.data[6])
	chars := rec.data[8:]

	u16 := make([]uint16, cch)
	for i := 0; i < len(chars) && i < cch; i++ {
		u16[i] = uint16(chars[i])
	}

	name = string(utf16.Decode(u16))
	return offset, sheetType, name, nil
}
