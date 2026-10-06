package xls

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf16"
)

// BIFF8 record type identifiers used by this package.
// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/6fba0383-0d7a-4c7a-afe9-642ff70cbd36
const (
	recEOF        = 0x000A
	recROW        = 0x0208
	recBOUNDSHEET = 0x0085
	recRK         = 0x027E
	recMULRK      = 0x00BD
	recLABELSST   = 0x00FD
	recBOOLERR    = 0x0205
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

func (rec biffRecord) parseBOUNDSHEET() (offset uint32, sheetType byte, name string, err error) {
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

// decodeRK decodes an RK-encoded number from a BIFF record into a float64 value.
// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/04fa5340-122f-49db-93ea-00cc75501efc
func decodeRK(rk uint32) float64 {
	fX100 := rk&0x1 != 0
	fInt := rk&0x2 != 0

	var value float64
	if fInt {
		value = float64(int32(rk) >> 2)
	} else {
		value = math.Float64frombits(uint64(rk>>2) << 34)
	}

	if fX100 {
		value /= 100
	}

	return value
}

func (rec biffRecord) parseRK() (row, col int, value float64, err error) {
	if len(rec.data) < 10 {
		return 0, 0, 0, errors.New("malformed RK record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	col = int(binary.LittleEndian.Uint16(rec.data[2:4]))
	rk := binary.LittleEndian.Uint32(rec.data[6:10])

	return row, col, decodeRK(rk), nil
}

func (rec biffRecord) parseBOOLERR() (row, col int, value bool, err error) {
	if len(rec.data) < 6 {
		return 0, 0, false, errors.New("malformed BOOLERR record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	col = int(binary.LittleEndian.Uint16(rec.data[2:4]))
	value = rec.data[6] != 0

	// TODO: handle error values
	// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/4a18edf4-b88c-4b39-a857-b31757314d0f

	return row, col, value, nil
}

func (rec biffRecord) parseMULRK() (row int, cols []int, values []float64, err error) {
	if len(rec.data) < 6 {
		return 0, nil, nil, errors.New("malformed MULRK record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	colFirst := int(binary.LittleEndian.Uint16(rec.data[2:4]))
	n := (len(rec.data) - 6) / 6
	cols = make([]int, n)
	values = make([]float64, n)
	for i := range n {
		// Each entry is {ixfe(2), rk(4)}; the ixfe field is skipped.
		base := 4 + i*6

		cols[i] = colFirst + i
		rk := binary.LittleEndian.Uint32(rec.data[base+2 : base+6])
		values[i] = decodeRK(rk)
	}
	return row, cols, values, nil
}
