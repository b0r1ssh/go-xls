package cfb_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/b0r1sh/go-xls/internal/cfb"
)

// baseHeader returns a minimal, well-formed 512-byte CFB header (version 3,
// 512-byte sectors, 64-byte mini sectors) that callers can further customize.
func baseHeader() []byte {
	header := make([]byte, 512)
	copy(header[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	header[26] = 0x03
	header[27] = 0x00 // majorVersion = 0x0003
	header[28] = 0xFE
	header[29] = 0xFF // byteOrder = 0xFFFE
	header[30] = 0x09
	header[31] = 0x00 // sectorShift = 0x0009
	header[32] = 0x06
	header[33] = 0x00 // miniSectorShift = 0x0006
	header[56] = 0x00
	header[57] = 0x00
	header[58] = 0x10
	header[59] = 0x00 // miniCutoff = 0x00100000
	return header
}

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("valid header", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()

		_, err := cfb.Open(bytes.NewReader(header))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid header with version 4 sector size", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		header[26] = 0x04
		header[27] = 0x00 // majorVersion = 0x0004
		header[30] = 0x0C
		header[31] = 0x00 // sectorShift = 0x000C

		_, err := cfb.Open(bytes.NewReader(header))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid header with DIFAT entries in header", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 3) // numFATSectors = 3
		binary.LittleEndian.PutUint32(header[76:80], 0)
		binary.LittleEndian.PutUint32(header[80:84], 1)
		binary.LittleEndian.PutUint32(header[84:88], 2)

		_, err := cfb.Open(bytes.NewReader(header))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid header with special sector stopping DIFAT scan early", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 5) // numFATSectors = 5
		binary.LittleEndian.PutUint32(header[76:80], 0)
		binary.LittleEndian.PutUint32(header[80:84], 0xFFFFFFFE) // special, stops the scan

		_, err := cfb.Open(bytes.NewReader(header))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid header with chained DIFAT sector", func(t *testing.T) {
		t.Parallel()

		const sectorSize = 512
		const entriesPerSector = sectorSize / 4

		header := baseHeader()
		// 109 entries fit inline in the header; ask for one more to force
		// the reader to follow the DIFAT sector chain.
		binary.LittleEndian.PutUint32(header[44:48], 110)
		for i := range 109 {
			binary.LittleEndian.PutUint32(header[76+i*4:80+i*4], uint32(i))
		}
		binary.LittleEndian.PutUint32(header[68:72], 0) // firstDIFATSector = 0
		binary.LittleEndian.PutUint32(header[72:76], 1) // numDIFATSectors = 1

		difatSector := make([]byte, sectorSize)
		for i := range entriesPerSector - 1 {
			binary.LittleEndian.PutUint32(difatSector[i*4:i*4+4], uint32(100+i))
		}
		binary.LittleEndian.PutUint32(difatSector[(entriesPerSector-1)*4:], 0xFFFFFFFE) // end of chain

		data := append(append([]byte{}, header...), difatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid header with self-referential DIFAT sector", func(t *testing.T) {
		t.Parallel()

		const sectorSize = 512

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 110) // numFATSectors, more than fits inline
		binary.LittleEndian.PutUint32(header[68:72], 0)   // firstDIFATSector = 0
		binary.LittleEndian.PutUint32(header[72:76], 1)   // numDIFATSectors = 1

		// A DIFAT sector made entirely of special entries yields no new IDs,
		// so the reader must stop instead of looping forever.
		difatSector := make([]byte, sectorSize)
		for i := range difatSector {
			difatSector[i] = 0xFF
		}

		data := append(append([]byte{}, header...), difatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("invalid header", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 512)
		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("empty header", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 0)
		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("invalid byte order", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 512)
		copy(header[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
		header[28] = 0x00
		header[29] = 0x00

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("unsupported version/sector size", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 512)
		copy(header[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
		header[26] = 0x00
		header[27] = 0x05 // majorVersion = 0x0005
		header[30] = 0x00
		header[31] = 0x0A // sectorShift = 0x000A

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("unsupported major version", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 512)
		copy(header[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
		header[26] = 0x00
		header[27] = 0x05 // majorVersion = 0x0005
		header[28] = 0xFE
		header[29] = 0xFF // byteOrder = 0xFFFE
		header[30] = 0x09
		header[31] = 0x00 // sectorShift = 0x0009

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("unsupported mini sector shift", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 512)
		copy(header[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
		header[26] = 0x00
		header[27] = 0x03 // majorVersion = 0x0003
		header[28] = 0xFE
		header[29] = 0xFF // byteOrder = 0xFFFE
		header[30] = 0x09
		header[31] = 0x00 // sectorShift = 0x0009
		header[32] = 0x07
		header[33] = 0x00 // miniSectorShift = 0x0007

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
