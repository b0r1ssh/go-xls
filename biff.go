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
	recNUMBER     = 0x0203
	recLABELSST   = 0x00FD
	recBOOLERR    = 0x0205
	recROW        = 0x0208
	recFORMAT     = 0x041E
	recXF         = 0x00E0
	recDATEMODE   = 0x0022
	recCONTINUE   = 0x003C
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
	flags := rec.data[7]
	fHighByte := flags&0x1 != 0
	chars := rec.data[8:]

	u16 := make([]uint16, 0, cch)
	if fHighByte {
		for i := 0; i+1 < len(chars) && len(u16) < cch; i += 2 {
			u16 = append(u16, binary.LittleEndian.Uint16(chars[i:i+2]))
		}
	} else {
		for i := 0; i < len(chars) && len(u16) < cch; i++ {
			u16 = append(u16, uint16(chars[i]))
		}
	}

	name = string(utf16.Decode(u16))
	return offset, sheetType, name, nil
}

// parseSST decodes the unique strings embedded directly in the SST record's
// data (XLUnicodeRichExtendedString entries), pulling CONTINUE records from
// rr for SST records too large to fit in a single BIFF record.
// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/3f52609d-816f-44a7-aad1-e0fe2abccebd
func (rec biffRecord) parseSST(rr *recordReader) (sst []string, err error) {
	if len(rec.data) < 8 {
		return nil, errors.New("malformed SST record")
	}

	cstUnique := binary.LittleEndian.Uint32(rec.data[4:8])
	sst = make([]string, 0, cstUnique)

	s := &sstReader{rr: rr, data: rec.data[8:]}
	for uint32(len(sst)) < cstUnique {
		hdr, err := s.readBytes(3)
		if err != nil {
			return nil, errors.New("malformed SST record: truncated string header")
		}

		cch := int(binary.LittleEndian.Uint16(hdr[0:2]))
		flags := hdr[2]

		fHighByte := flags&0x1 != 0
		fExtSt := flags&0x4 != 0
		fRichSt := flags&0x8 != 0

		crun := 0
		if fRichSt {
			b, err := s.readBytes(2)
			if err != nil {
				return nil, errors.New("malformed SST record: truncated rich string header")
			}

			crun = int(binary.LittleEndian.Uint16(b))
		}

		cbExtRst := 0
		if fExtSt {
			b, err := s.readBytes(4)
			if err != nil {
				return nil, errors.New("malformed SST record: truncated ext string header")
			}

			cbExtRst = int(binary.LittleEndian.Uint32(b))
		}

		str, err := s.readChars(cch, fHighByte)
		if err != nil {
			return nil, errors.New("malformed SST record: truncated string data")
		}

		if _, err := s.readBytes(crun*4 + cbExtRst); err != nil {
			return nil, errors.New("malformed SST record: truncated string trailer")
		}

		sst = append(sst, str)
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

func (rec biffRecord) parseNUMBER() (row, col int, ixfe uint16, value float64, err error) {
	if len(rec.data) < 14 {
		return 0, 0, 0, 0, errors.New("malformed NUMBER record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	col = int(binary.LittleEndian.Uint16(rec.data[2:4]))
	ixfe = binary.LittleEndian.Uint16(rec.data[4:6])
	bits := binary.LittleEndian.Uint64(rec.data[6:14])
	value = math.Float64frombits(bits)

	return row, col, ixfe, value, nil
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

func (rec biffRecord) parseROW() (row int, firstCol, lastCol uint16, err error) {
	if len(rec.data) < 6 {
		return 0, 0, 0, errors.New("malformed ROW record")
	}

	row = int(binary.LittleEndian.Uint16(rec.data[0:2]))
	firstCol = binary.LittleEndian.Uint16(rec.data[2:4])
	lastCol = binary.LittleEndian.Uint16(rec.data[4:6]) - 1

	return row, firstCol, lastCol, nil
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
