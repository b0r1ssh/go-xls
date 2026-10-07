package xls

import (
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

// sstReader reads bytes for an SST record's body, transparently pulling
// subsequent CONTINUE records from rr once the current record's data is
// exhausted (BIFF8 records are capped at ~8KB, so large SST records spill
// into CONTINUE records).
// https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/173d9f51-e5d3-43da-8de2-be7f22e119b9
type sstReader struct {
	rr   *recordReader
	data []byte
}

func (s *sstReader) fill() error {
	if len(s.data) > 0 {
		return nil
	}

	rec, err := s.rr.next()
	if err != nil {
		return err
	}

	if rec.recType != recCONTINUE {
		return errors.New("expected CONTINUE record")
	}

	s.data = rec.data
	return nil
}

func (s *sstReader) readBytes(n int) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}

	if err := s.fill(); err != nil {
		return nil, err
	}

	if len(s.data) >= n {
		b := s.data[:n]
		s.data = s.data[n:]
		return b, nil
	}

	buf := make([]byte, 0, n)
	for len(buf) < n {
		if err := s.fill(); err != nil {
			return nil, err
		}

		take := min(n-len(buf), len(s.data))
		buf = append(buf, s.data[:take]...)
		s.data = s.data[take:]
	}
	return buf, nil
}

func (s *sstReader) readChars(cch int, fHighByte bool) (string, error) {
	u16 := make([]uint16, 0, cch)
	for len(u16) < cch {
		if len(s.data) == 0 {
			rec, err := s.rr.next()
			if err != nil {
				return "", err
			}

			if rec.recType != recCONTINUE || len(rec.data) < 1 {
				return "", errors.New("expected CONTINUE record")
			}

			fHighByte = rec.data[0]&0x1 != 0
			s.data = rec.data[1:]
			continue
		}

		if fHighByte {
			b, err := s.readBytes(2)
			if err != nil {
				return "", err
			}

			u16 = append(u16, binary.LittleEndian.Uint16(b))
		} else {
			u16 = append(u16, uint16(s.data[0]))
			s.data = s.data[1:]
		}
	}
	return string(utf16.Decode(u16)), nil
}
