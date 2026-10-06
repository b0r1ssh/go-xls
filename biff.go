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
	recBOUNDSHEET = 0x0085
	recSST        = 0x00FC
	recRK         = 0x027E
	recMULRK      = 0x00BD
	recLABELSST   = 0x00FD
	recBOOLERR    = 0x0205
	recFORMAT     = 0x041E
	recXF         = 0x00E0
	recDATEMODE   = 0x0022
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

// parseSST decodes the unique strings embedded directly in the SST record's
// data (XLUnicodeRichExtendedString entries), not as separate BIFF records.
// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/3f52609d-816f-44a7-aad1-e0fe2abccebd
func (rec biffRecord) parseSST() (sst []string, err error) {
	if len(rec.data) < 8 {
		return nil, errors.New("malformed SST record")
	}

	cstUnique := binary.LittleEndian.Uint32(rec.data[4:8])
	sst = make([]string, 0, cstUnique)

	data := rec.data[8:]
	for uint32(len(sst)) < cstUnique {
		if len(data) < 3 {
			return nil, errors.New("malformed SST record: truncated string header")
		}

		cch := int(binary.LittleEndian.Uint16(data[0:2]))
		flags := data[2]
		data = data[3:]

		fHighByte := flags&0x1 != 0
		fExtSt := flags&0x4 != 0
		fRichSt := flags&0x8 != 0

		crun := 0
		if fRichSt {
			if len(data) < 2 {
				return nil, errors.New("malformed SST record: truncated rich string header")
			}
			crun = int(binary.LittleEndian.Uint16(data[0:2]))
			data = data[2:]
		}

		cbExtRst := 0
		if fExtSt {
			if len(data) < 4 {
				return nil, errors.New("malformed SST record: truncated ext string header")
			}
			cbExtRst = int(binary.LittleEndian.Uint32(data[0:4]))
			data = data[4:]
		}

		charBytes := cch
		if fHighByte {
			charBytes = cch * 2
		}
		if len(data) < charBytes {
			return nil, errors.New("malformed SST record: truncated string data")
		}

		var s string
		if fHighByte {
			u16 := make([]uint16, cch)
			for i := range u16 {
				u16[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
			}
			s = string(utf16.Decode(u16))
		} else {
			s = string(data[:cch])
		}
		data = data[charBytes:]

		runBytes := crun * 4
		if len(data) < runBytes+cbExtRst {
			return nil, errors.New("malformed SST record: truncated string trailer")
		}
		data = data[runBytes+cbExtRst:]

		sst = append(sst, s)
	}

	return sst, nil
}

func (rec biffRecord) parseRK() (row, col int, ixfe uint16, value float64, err error) {
	if len(rec.data) < 10 {
		return 0, 0, 0, 0, errors.New("malformed RK record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	col = int(binary.LittleEndian.Uint16(rec.data[2:4]))
	ixfe = binary.LittleEndian.Uint16(rec.data[4:6])
	rk := binary.LittleEndian.Uint32(rec.data[6:10])

	return row, col, ixfe, decodeRK(rk), nil
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

func (rec biffRecord) parseMULRK() (row int, cols []int, ixfes []uint16, values []float64, err error) {
	if len(rec.data) < 6 {
		return 0, nil, nil, nil, errors.New("malformed MULRK record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	colFirst := int(binary.LittleEndian.Uint16(rec.data[2:4]))
	n := (len(rec.data) - 6) / 6
	cols = make([]int, n)
	ixfes = make([]uint16, n)
	values = make([]float64, n)
	for i := range n {
		// Each entry is {ixfe(2), rk(4)}.
		base := 4 + i*6

		cols[i] = colFirst + i
		ixfes[i] = binary.LittleEndian.Uint16(rec.data[base : base+2])
		rk := binary.LittleEndian.Uint32(rec.data[base+2 : base+6])
		values[i] = decodeRK(rk)
	}
	return row, cols, ixfes, values, nil
}

func (rec biffRecord) parseLABELSST(sst []string) (row, col int, value string, err error) {
	if len(rec.data) < 10 {
		return 0, 0, "", errors.New("malformed LABELSST record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	col = int(binary.LittleEndian.Uint16(rec.data[2:4]))

	idx := binary.LittleEndian.Uint32(rec.data[6:10])
	s := ""
	if int(idx) < len(sst) {
		s = sst[idx]
	}

	return row, col, s, nil
}

func (rec biffRecord) parseFORMAT() (ifmt uint16, code string, err error) {
	if len(rec.data) < 5 {
		return 0, "", errors.New("malformed FORMAT record")
	}

	ifmt = binary.LittleEndian.Uint16(rec.data[0:2])
	cch := int(binary.LittleEndian.Uint16(rec.data[2:4]))
	fHighByte := rec.data[4]&0x1 != 0
	data := rec.data[5:]

	charBytes := cch
	if fHighByte {
		charBytes = cch * 2
	}
	if len(data) < charBytes {
		return 0, "", errors.New("malformed FORMAT record: truncated string data")
	}

	if fHighByte {
		u16 := make([]uint16, cch)
		for i := range u16 {
			u16[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
		}
		code = string(utf16.Decode(u16))
	} else {
		code = string(data[:cch])
	}

	return ifmt, code, nil
}

func (rec biffRecord) parseXF() (ifmt uint16, err error) {
	if len(rec.data) < 4 {
		return 0, errors.New("malformed XF record")
	}

	ifmt = binary.LittleEndian.Uint16(rec.data[2:4])

	return ifmt, nil
}

func (rec biffRecord) parseDATEMODE() (is1904 bool, err error) {
	if len(rec.data) < 2 {
		return false, errors.New("malformed 1904 record")
	}

	is1904 = binary.LittleEndian.Uint16(rec.data[0:2]) != 0

	return is1904, nil
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
