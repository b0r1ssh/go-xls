package cfb

import (
	"errors"
	"io"
)

type Stream struct {
	r *Reader

	mini bool

	chain []uint32

	size int64
	pos  int64
}

// ReadAt implements io.ReaderAt.
func (s *Stream) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative read offset")
	}

	if off >= s.size {
		return 0, io.EOF
	}

	want := len(p)
	if off+int64(want) > s.size {
		p = p[:s.size-off]
	}

	secSize := int64(s.r.sectorSize)
	if s.mini {
		secSize = int64(s.r.miniSectorSize)
	}

	n := 0
	for n < len(p) {
		cur := off + int64(n)
		idx := int(cur / secSize)
		secID := s.chain[idx]
		inSec := int(cur % secSize)
		toRead := min(int(secSize)-inSec, len(p)-n)

		var src []byte
		if s.mini {
			rootOffset := int64(secID)*int64(s.r.miniSectorSize) + int64(inSec)
			mainSecSize := s.r.sectorSize
			mainIdx := int(rootOffset / int64(mainSecSize))
			within := int(rootOffset % int64(mainSecSize))
			if mainIdx >= len(s.r.rootChain) {
				return n, io.ErrUnexpectedEOF
			}

			sec, err := readSector(s.r.ra, s.r.rootChain[mainIdx], mainSecSize)
			if err != nil {
				return n, err
			}

			if within >= len(sec) {
				break
			}

			src = sec[within:]
		} else {
			sec, err := readSector(s.r.ra, secID, s.r.sectorSize)
			if err != nil {
				return n, err
			}

			if inSec >= len(sec) {
				break
			}

			src = sec[inSec:]
		}
		toRead = min(toRead, len(src))
		copy(p[n:n+toRead], src[:toRead])
		n += toRead
	}
	if n < len(p) {
		return n, io.ErrUnexpectedEOF
	}

	if n < want {
		return n, io.EOF
	}

	return n, nil
}

// Read implements io.Reader, advancing the stream's internal cursor.
func (s *Stream) Read(p []byte) (int, error) {
	n, err := s.ReadAt(p, s.pos)
	s.pos += int64(n)
	return n, err
}

// Seek implements io.Seeker.
func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	var pos int64
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = s.pos + offset
	case io.SeekEnd:
		pos = s.size + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if pos < 0 {
		return 0, errors.New("negative seek position")
	}

	s.pos = pos
	return pos, nil
}

func readSector(ra io.ReaderAt, sector uint32, secSize int) ([]byte, error) {
	buf := make([]byte, secSize)
	off := (int64(sector) + 1) * int64(secSize)
	n, err := ra.ReadAt(buf, off)
	if err != nil && !(n > 0 && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF))) {
		return nil, err
	}
	return buf[:n], nil
}
