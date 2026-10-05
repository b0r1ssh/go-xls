package cfb_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/b0r1sh/go-xls/internal/cfb"
)

// newTestStream builds a minimal CFB image with a single two-sector stream
// named "DATA" and returns the opened stream alongside its expected content.
func newTestStream(t *testing.T) (*cfb.Stream, []byte) {
	t.Helper()

	const sectorSize = 512

	want := make([]byte, 600)
	for i := range want {
		want[i] = byte(i)
	}

	header := baseHeader()
	binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
	binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
	binary.LittleEndian.PutUint32(header[56:60], 0) // miniCutoff = 0, forcing regular (non-mini) streams

	// sector 0 holds the FAT: it ends the root directory chain at sector 1
	// and chains the stream's data across sectors 2 and 3.
	fatSector := make([]byte, sectorSize)
	binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(fatSector[8:12], 3)
	binary.LittleEndian.PutUint32(fatSector[12:16], 0xFFFFFFFE)

	// sector 1 holds the directory: slot 0 is the root entry, slot 1 is the
	// "DATA" stream starting at sector 2.
	dirSector := make([]byte, sectorSize)
	dirSector[66] = 5                                             // object type = root storage
	binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE) // root stream is empty

	binary.LittleEndian.PutUint16(dirSector[128:130], 0x0044)    // "D"
	binary.LittleEndian.PutUint16(dirSector[130:132], 0x0041)    // "A"
	binary.LittleEndian.PutUint16(dirSector[132:134], 0x0054)    // "T"
	binary.LittleEndian.PutUint16(dirSector[134:136], 0x0041)    // "A"
	binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 10)  // nameLen, including the null terminator
	dirSector[128+66] = 2                                        // object type = stream
	binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 2) // start sector = 2
	binary.LittleEndian.PutUint64(dirSector[128+120:128+128], uint64(len(want)))

	// sectors 2 and 3 hold the stream's content.
	contentSectors := make([]byte, sectorSize*2)
	copy(contentSectors, want)

	data := append(append(append(append([]byte{}, header...), fatSector...), dirSector...), contentSectors...)

	r, err := cfb.Open(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	s, err := r.Stream("DATA")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	return s, want
}

// newMiniStreamData builds a CFB image with a mini stream named "M" whose
// single mini-sector lives at logical mini-sector id miniStart, backed by
// rootData within the root entry's one-sector regular stream (sector 3).
// rootData shorter than a full sector simulates a truncated/corrupt file.
func newMiniStreamData(miniStart uint32, size uint64, rootData []byte) []byte {
	const sectorSize = 512

	header := baseHeader()
	binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
	binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
	binary.LittleEndian.PutUint32(header[60:64], 2) // firstMiniFATSector = 2
	binary.LittleEndian.PutUint32(header[64:68], 1) // numMiniFATSectors = 1

	// sector 0 holds the FAT: it ends the directory chain at sector 1, the
	// mini FAT chain at sector 2, and the root entry's data chain at sector 3.
	fatSector := make([]byte, sectorSize)
	binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(fatSector[8:12], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(fatSector[12:16], 0xFFFFFFFE)

	// sector 1 holds the directory: slot 0 is the root entry (backed by
	// sector 3), slot 1 is the mini stream named "M".
	dirSector := make([]byte, sectorSize)
	dirSector[66] = 5                                    // object type = root storage
	binary.LittleEndian.PutUint32(dirSector[116:120], 3) // root data starts at sector 3
	binary.LittleEndian.PutUint64(dirSector[120:128], sectorSize)

	binary.LittleEndian.PutUint16(dirSector[128:130], 0x004D)            // "M"
	binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)           // nameLen, including the null terminator
	dirSector[128+66] = 2                                                // object type = stream
	binary.LittleEndian.PutUint32(dirSector[128+116:128+120], miniStart) // start mini-sector
	binary.LittleEndian.PutUint64(dirSector[128+120:128+128], size)      // stream size

	// sector 2 holds the mini FAT; the stream's single mini-sector ends its chain.
	miniFATSector := make([]byte, sectorSize)
	binary.LittleEndian.PutUint32(miniFATSector[miniStart*4:miniStart*4+4], 0xFFFFFFFE)

	data := append(append(append([]byte{}, header...), fatSector...), dirSector...)
	data = append(data, miniFATSector...)
	data = append(data, rootData...)
	return data
}

// failingReaderAt wraps a byte slice but fails with a fixed error at a
// chosen offset, simulating a disk I/O error on a specific sector read.
type failingReaderAt struct {
	data    []byte
	failOff int64
	err     error
}

func (f *failingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off == f.failOff {
		return 0, f.err
	}
	return bytes.NewReader(f.data).ReadAt(p, off)
}

func TestStreamReadAt(t *testing.T) {
	t.Parallel()

	t.Run("reads from the start", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		got := make([]byte, 16)
		n, err := s.ReadAt(got, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if n != len(got) {
			t.Fatalf("expected %d bytes, got %d", len(got), n)
		}
		if !bytes.Equal(got, want[:16]) {
			t.Fatalf("expected %v, got %v", want[:16], got)
		}
	})

	t.Run("reads across a sector boundary", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		got := make([]byte, 32)
		n, err := s.ReadAt(got, 500)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if n != len(got) {
			t.Fatalf("expected %d bytes, got %d", len(got), n)
		}
		if !bytes.Equal(got, want[500:532]) {
			t.Fatalf("expected %v, got %v", want[500:532], got)
		}
	})

	t.Run("short read at end of stream", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		got := make([]byte, 16)
		n, err := s.ReadAt(got, int64(len(want))-8)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF, got %v", err)
		}
		if n != 8 {
			t.Fatalf("expected 8 bytes, got %d", n)
		}
		if !bytes.Equal(got[:n], want[len(want)-8:]) {
			t.Fatalf("expected %v, got %v", want[len(want)-8:], got[:n])
		}
	})

	t.Run("offset at or past the end", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		got := make([]byte, 16)
		n, err := s.ReadAt(got, int64(len(want)))
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF, got %v", err)
		}
		if n != 0 {
			t.Fatalf("expected 0 bytes, got %d", n)
		}
	})

	t.Run("negative offset", func(t *testing.T) {
		t.Parallel()

		s, _ := newTestStream(t)

		_, err := s.ReadAt(make([]byte, 1), -1)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("mini stream reads real data", func(t *testing.T) {
		t.Parallel()

		want := make([]byte, 10)
		for i := range want {
			want[i] = byte(i)
		}
		rootData := make([]byte, 512)
		copy(rootData, want)

		r, err := cfb.Open(bytes.NewReader(newMiniStreamData(0, uint64(len(want)), rootData)))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("M")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		got := make([]byte, len(want))
		n, err := s.ReadAt(got, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if n != len(want) {
			t.Fatalf("expected %d bytes, got %d", len(want), n)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("mini stream root chain out of range", func(t *testing.T) {
		t.Parallel()

		// mini-sector 8 maps to the second root-stream sector, but the root
		// entry's chain only covers one.
		data := newMiniStreamData(8, 10, make([]byte, 512))

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("M")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = s.ReadAt(make([]byte, 10), 0)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("mini stream truncated root sector", func(t *testing.T) {
		t.Parallel()

		rootData := []byte{0, 1, 2, 3, 4} // shorter than the mini-sector's own size
		data := newMiniStreamData(0, 10, rootData)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("M")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		got := make([]byte, 10)
		n, err := s.ReadAt(got, 0)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
		}
		if n != len(rootData) {
			t.Fatalf("expected %d bytes, got %d", len(rootData), n)
		}
		if !bytes.Equal(got[:n], rootData) {
			t.Fatalf("expected %v, got %v", rootData, got[:n])
		}
	})

	t.Run("mini stream read error", func(t *testing.T) {
		t.Parallel()

		data := newMiniStreamData(0, 10, make([]byte, 512))
		boom := errors.New("boom")
		ra := &failingReaderAt{data: data, failOff: 4 * 512, err: boom} // sector 3's offset

		r, err := cfb.Open(ra)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("M")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = s.ReadAt(make([]byte, 10), 0)
		if !errors.Is(err, boom) {
			t.Fatalf("expected %v, got %v", boom, err)
		}
	})

	t.Run("regular stream read error", func(t *testing.T) {
		t.Parallel()

		const sectorSize = 512

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
		binary.LittleEndian.PutUint32(header[56:60], 0) // miniCutoff = 0, forcing a regular stream

		fatSector := make([]byte, sectorSize)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
		binary.LittleEndian.PutUint32(fatSector[8:12], 0xFFFFFFFE)

		dirSector := make([]byte, sectorSize)
		dirSector[66] = 5
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE)

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0045)    // "E"
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)   // nameLen, including the null terminator
		dirSector[128+66] = 2                                        // object type = stream
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 2) // start sector = 2
		binary.LittleEndian.PutUint64(dirSector[128+120:128+128], 10)

		data := append(append(append([]byte{}, header...), fatSector...), dirSector...)
		data = append(data, make([]byte, sectorSize)...) // sector 2, content irrelevant

		boom := errors.New("boom")
		ra := &failingReaderAt{data: data, failOff: 3 * sectorSize, err: boom} // sector 2's offset

		r, err := cfb.Open(ra)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("E")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = s.ReadAt(make([]byte, 10), 0)
		if !errors.Is(err, boom) {
			t.Fatalf("expected %v, got %v", boom, err)
		}
	})

	t.Run("regular stream unexpected EOF on truncated sector", func(t *testing.T) {
		t.Parallel()

		const sectorSize = 512

		want := make([]byte, 600)
		for i := range want {
			want[i] = byte(i)
		}

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
		binary.LittleEndian.PutUint32(header[56:60], 0) // miniCutoff = 0, forcing a regular stream

		// sector 0 holds the FAT, chaining the stream's data across sectors 2 and 3.
		fatSector := make([]byte, sectorSize)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
		binary.LittleEndian.PutUint32(fatSector[8:12], 3)
		binary.LittleEndian.PutUint32(fatSector[12:16], 0xFFFFFFFE)

		dirSector := make([]byte, sectorSize)
		dirSector[66] = 5
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE)

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0044)
		binary.LittleEndian.PutUint16(dirSector[130:132], 0x0041)
		binary.LittleEndian.PutUint16(dirSector[132:134], 0x0054)
		binary.LittleEndian.PutUint16(dirSector[134:136], 0x0041)
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 10)
		dirSector[128+66] = 2
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 2)
		binary.LittleEndian.PutUint64(dirSector[128+120:128+128], uint64(len(want)))

		data := append(append(append([]byte{}, header...), fatSector...), dirSector...)
		data = append(data, want[:sectorSize]...)              // sector 2, full
		data = append(data, want[sectorSize:sectorSize+50]...) // sector 3, truncated to 50 bytes

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("DATA")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		got := make([]byte, len(want))
		n, err := s.ReadAt(got, 0)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
		}
		wantN := sectorSize + 50
		if n != wantN {
			t.Fatalf("expected %d bytes, got %d", wantN, n)
		}
		if !bytes.Equal(got[:n], want[:n]) {
			t.Fatalf("expected %v, got %v", want[:n], got[:n])
		}
	})
}

func TestStreamRead(t *testing.T) {
	t.Parallel()

	t.Run("sequential reads return the full stream", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		got, err := io.ReadAll(s)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("advances the cursor across calls", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		first := make([]byte, 10)
		if _, err := s.Read(first); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		second := make([]byte, 10)
		if _, err := s.Read(second); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !bytes.Equal(first, want[:10]) {
			t.Fatalf("expected %v, got %v", want[:10], first)
		}
		if !bytes.Equal(second, want[10:20]) {
			t.Fatalf("expected %v, got %v", want[10:20], second)
		}
	})
}

func TestStreamSeek(t *testing.T) {
	t.Parallel()

	t.Run("seek start", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		pos, err := s.Seek(100, io.SeekStart)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pos != 100 {
			t.Fatalf("expected position 100, got %d", pos)
		}

		got := make([]byte, 10)
		if _, err := s.Read(got); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !bytes.Equal(got, want[100:110]) {
			t.Fatalf("expected %v, got %v", want[100:110], got)
		}
	})

	t.Run("seek current", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		if _, err := s.Seek(50, io.SeekStart); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		pos, err := s.Seek(20, io.SeekCurrent)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pos != 70 {
			t.Fatalf("expected position 70, got %d", pos)
		}

		got := make([]byte, 10)
		if _, err := s.Read(got); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !bytes.Equal(got, want[70:80]) {
			t.Fatalf("expected %v, got %v", want[70:80], got)
		}
	})

	t.Run("seek end", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		pos, err := s.Seek(-8, io.SeekEnd)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pos != int64(len(want))-8 {
			t.Fatalf("expected position %d, got %d", len(want)-8, pos)
		}

		got := make([]byte, 8)
		if _, err := s.Read(got); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !bytes.Equal(got, want[len(want)-8:]) {
			t.Fatalf("expected %v, got %v", want[len(want)-8:], got)
		}
	})

	t.Run("negative position", func(t *testing.T) {
		t.Parallel()

		s, _ := newTestStream(t)

		_, err := s.Seek(-1, io.SeekStart)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("invalid whence", func(t *testing.T) {
		t.Parallel()

		s, _ := newTestStream(t)

		_, err := s.Seek(0, 99)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestStreamSize(t *testing.T) {
	t.Parallel()

	t.Run("regular stream", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		if got := s.Size(); got != int64(len(want)) {
			t.Fatalf("expected size %d, got %d", len(want), got)
		}
	})

	t.Run("mini stream", func(t *testing.T) {
		t.Parallel()

		const size = 10

		rootData := make([]byte, 512)
		data := newMiniStreamData(0, size, rootData)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		s, err := r.Stream("M")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := s.Size(); got != size {
			t.Fatalf("expected size %d, got %d", size, got)
		}
	})

	t.Run("unaffected by reads and seeks", func(t *testing.T) {
		t.Parallel()

		s, want := newTestStream(t)

		if _, err := s.Seek(100, io.SeekStart); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if _, err := s.Read(make([]byte, 10)); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := s.Size(); got != int64(len(want)) {
			t.Fatalf("expected size %d, got %d", len(want), got)
		}
	})
}
