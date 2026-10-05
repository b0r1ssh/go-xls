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
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1

		// sector 0 holds the FAT; its first entry ends the root directory chain.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[0:4], 0xFFFFFFFE)

		// sector 0 doubles as the directory sector; slot 1 holds the root entry.
		fatSector[128+66] = 5                                                 // object type = root storage
		binary.LittleEndian.PutUint32(fatSector[128+116:128+120], 0xFFFFFFFE) // root stream is empty

		data := append(append([]byte{}, header...), fatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
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
		header[31] = 0x00                               // sectorShift = 0x000C
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1

		const sectorSize = 4096

		// sector 0 holds the FAT; its first entry ends the root directory chain.
		fatSector := make([]byte, sectorSize)
		binary.LittleEndian.PutUint32(fatSector[0:4], 0xFFFFFFFE)

		// sector 0 doubles as the directory sector; slot 1 holds the root entry.
		fatSector[128+66] = 5                                                 // object type = root storage
		binary.LittleEndian.PutUint32(fatSector[128+116:128+120], 0xFFFFFFFE) // root stream is empty

		data := make([]byte, sectorSize+sectorSize)
		copy(data, header)
		copy(data[sectorSize:], fatSector)

		_, err := cfb.Open(bytes.NewReader(data))
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

		// sector 0 holds the FAT; its first entry ends the root directory chain.
		// sectors 1 and 2 only need to be present.
		sectors := make([]byte, 512*3)
		binary.LittleEndian.PutUint32(sectors[0:4], 0xFFFFFFFE)

		// sector 0 doubles as the directory sector; slot 1 holds the root entry.
		sectors[128+66] = 5                                                 // object type = root storage
		binary.LittleEndian.PutUint32(sectors[128+116:128+120], 0xFFFFFFFE) // root stream is empty

		data := append(append([]byte{}, header...), sectors...)

		_, err := cfb.Open(bytes.NewReader(data))
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

		// sector 0 holds the FAT; its first entry ends the root directory chain.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[0:4], 0xFFFFFFFE)

		// sector 0 doubles as the directory sector; slot 1 holds the root entry.
		fatSector[128+66] = 5                                                 // object type = root storage
		binary.LittleEndian.PutUint32(fatSector[128+116:128+120], 0xFFFFFFFE) // root stream is empty

		data := append(append([]byte{}, header...), fatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
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
		// sector 0 doubles as a FAT sector, so its last entry (the DIFAT chain
		// terminator below) also ends the root directory chain.
		binary.LittleEndian.PutUint32(header[48:52], entriesPerSector-1)

		difatSector := make([]byte, sectorSize)
		for i := range entriesPerSector - 1 {
			binary.LittleEndian.PutUint32(difatSector[i*4:i*4+4], uint32(100+i))
		}
		binary.LittleEndian.PutUint32(difatSector[(entriesPerSector-1)*4:], 0xFFFFFFFE) // end of chain

		// pad so every FAT sector referenced by the DIFAT (up to id 226) is present.
		data := make([]byte, 228*sectorSize)
		copy(data, header)
		copy(data[sectorSize:], difatSector)

		// the root directory chain resolves to sector 127; give it a root entry.
		dirSector := make([]byte, sectorSize)
		dirSector[66] = 5                                             // object type = root storage
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE) // root stream is empty
		copy(data[(entriesPerSector-1+1)*sectorSize:], dirSector)

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
		binary.LittleEndian.PutUint32(header[48:52], 1)   // firstDirSector = 1

		// A DIFAT sector made entirely of special entries yields no new IDs,
		// so the reader must stop instead of looping forever.
		difatSector := make([]byte, sectorSize)
		for i := range difatSector {
			difatSector[i] = 0xFF
		}

		// sector 1 holds the directory, separate from the all-special DIFAT sector.
		dirSector := make([]byte, sectorSize)
		dirSector[66] = 5                                             // object type = root storage
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE) // root stream is empty

		data := append(append(append([]byte{}, header...), difatSector...), dirSector...)

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
		header[26] = 0x05
		header[27] = 0x00 // majorVersion = 0x0005
		header[30] = 0x0A
		header[31] = 0x00 // sectorShift = 0x000A

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("unsupported major version", func(t *testing.T) {
		t.Parallel()

		header := make([]byte, 512)
		copy(header[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
		header[26] = 0x05
		header[27] = 0x00 // majorVersion = 0x0005
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
		header[26] = 0x03
		header[27] = 0x00 // majorVersion = 0x0003
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

	t.Run("directory sector out of range", func(t *testing.T) {
		t.Parallel()

		// numFATSectors defaults to 0, so the FAT is empty and the root
		// directory sector (0) has no entry to resolve.
		header := baseHeader()

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("directory sector with reserved id", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[48:52], 0xFFFFFFFB) // firstDirSector = reserved

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("circular directory chain", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1

		// sector 0 holds the FAT; its first entry points back to itself.
		fatSector := make([]byte, 512)
		data := append(append([]byte{}, header...), fatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("DIFAT sector with reserved id", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 200)        // numFATSectors, forces a DIFAT sector lookup
		binary.LittleEndian.PutUint32(header[68:72], 0xFFFFFFFB) // firstDIFATSector = reserved
		binary.LittleEndian.PutUint32(header[72:76], 1)          // numDIFATSectors = 1

		_, err := cfb.Open(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("FAT sector read failure", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 2) // numFATSectors = 2
		binary.LittleEndian.PutUint32(header[76:80], 0)
		binary.LittleEndian.PutUint32(header[80:84], 1) // second FAT sector is never written

		// only sector 0 is present; sector 1 is missing, so building the FAT fails.
		fatSector := make([]byte, 512)
		data := append(append([]byte{}, header...), fatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("directory sector read failure", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1

		// sector 0 holds the FAT and ends the root directory chain at sector 1,
		// but sector 1 itself is never written.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
		data := append(append([]byte{}, header...), fatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("root entry not found", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1

		// sector 0 holds the FAT and ends the root directory chain at sector 0;
		// its directory slots are left unallocated, so no root entry exists.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[0:4], 0xFFFFFFFE)
		data := append(append([]byte{}, header...), fatSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("directory entry with name", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1

		// sector 0 holds the FAT and ends the root directory chain at sector 1.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)

		// sector 1 holds the directory; its first entry has a 1-character name.
		dirSector := make([]byte, 512)
		binary.LittleEndian.PutUint16(dirSector[0:2], 0x0041)         // name = "A"
		binary.LittleEndian.PutUint16(dirSector[64:66], 4)            // nameLen, including the null terminator
		dirSector[66] = 5                                             // object type = root storage
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE) // root stream is empty

		data := append(append(append([]byte{}, header...), fatSector...), dirSector...)

		_, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestStream(t *testing.T) {
	t.Parallel()

	t.Run("basic stream retrieval", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1

		// sector 0 holds the FAT and ends the root directory chain at sector 1.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)

		// sector 1 holds the directory: slot 0 is the root entry, slot 1 is a
		// stream named "A".
		dirSector := make([]byte, 512)
		dirSector[66] = 5                                             // object type = root storage
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE) // root stream is empty

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0041)             // name = "A"
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)            // nameLen, including the null terminator
		dirSector[128+66] = 2                                                 // object type = stream
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 0xFFFFFFFE) // stream is empty

		data := append(append(append([]byte{}, header...), fatSector...), dirSector...)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = r.Stream("A")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("mini stream retrieval", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
		binary.LittleEndian.PutUint32(header[60:64], 2) // firstMiniFATSector = 2
		binary.LittleEndian.PutUint32(header[64:68], 1) // numMiniFATSectors = 1

		// sector 0 holds the FAT: it ends the root directory chain at sector 1
		// and the mini FAT chain at sector 2.
		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
		binary.LittleEndian.PutUint32(fatSector[8:12], 0xFFFFFFFE)

		// sector 1 holds the directory: slot 0 is the root entry, slot 1 is a
		// stream named "A" small enough to live in the mini stream.
		dirSector := make([]byte, 512)
		dirSector[66] = 5                                             // object type = root storage
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE) // root stream is empty

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0041)     // name = "A"
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)    // nameLen, including the null terminator
		dirSector[128+66] = 2                                         // object type = stream
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 0)  // start mini-sector = 0
		binary.LittleEndian.PutUint64(dirSector[128+120:128+128], 10) // stream size = 10 bytes

		// sector 2 holds the mini FAT: mini-sector 0 ends the stream's chain.
		miniFATSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(miniFATSector[0:4], 0xFFFFFFFE)

		data := append(append(append(append([]byte{}, header...), fatSector...), dirSector...), miniFATSector...)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = r.Stream("A")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("mini stream with reserved start id", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
		binary.LittleEndian.PutUint32(header[60:64], 2) // firstMiniFATSector = 2
		binary.LittleEndian.PutUint32(header[64:68], 1) // numMiniFATSectors = 1

		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
		binary.LittleEndian.PutUint32(fatSector[8:12], 0xFFFFFFFE)

		dirSector := make([]byte, 512)
		dirSector[66] = 5
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE)

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0041)
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)
		dirSector[128+66] = 2
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 0xFFFFFFFB) // reserved mini-sector id
		binary.LittleEndian.PutUint64(dirSector[128+120:128+128], 10)

		miniFATSector := make([]byte, 512)

		data := append(append(append(append([]byte{}, header...), fatSector...), dirSector...), miniFATSector...)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = r.Stream("A")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("mini stream start out of range", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
		// no mini FAT sectors are configured, so r.miniFAT stays empty.

		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)

		dirSector := make([]byte, 512)
		dirSector[66] = 5
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE)

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0041)
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)
		dirSector[128+66] = 2
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 0) // mini-sector 0, but miniFAT is empty
		binary.LittleEndian.PutUint64(dirSector[128+120:128+128], 10)

		data := append(append(append([]byte{}, header...), fatSector...), dirSector...)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = r.Stream("A")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("mini stream circular chain", func(t *testing.T) {
		t.Parallel()

		header := baseHeader()
		binary.LittleEndian.PutUint32(header[44:48], 1) // numFATSectors = 1
		binary.LittleEndian.PutUint32(header[48:52], 1) // firstDirSector = 1
		binary.LittleEndian.PutUint32(header[60:64], 2) // firstMiniFATSector = 2
		binary.LittleEndian.PutUint32(header[64:68], 1) // numMiniFATSectors = 1

		fatSector := make([]byte, 512)
		binary.LittleEndian.PutUint32(fatSector[4:8], 0xFFFFFFFE)
		binary.LittleEndian.PutUint32(fatSector[8:12], 0xFFFFFFFE)

		dirSector := make([]byte, 512)
		dirSector[66] = 5
		binary.LittleEndian.PutUint32(dirSector[116:120], 0xFFFFFFFE)

		binary.LittleEndian.PutUint16(dirSector[128:130], 0x0041)
		binary.LittleEndian.PutUint16(dirSector[128+64:128+66], 4)
		dirSector[128+66] = 2
		binary.LittleEndian.PutUint32(dirSector[128+116:128+120], 0) // mini-sector 0 points back to itself
		binary.LittleEndian.PutUint64(dirSector[128+120:128+128], 10)

		// mini-sector 0's entry (left zero) points back to mini-sector 0.
		miniFATSector := make([]byte, 512)

		data := append(append(append(append([]byte{}, header...), fatSector...), dirSector...), miniFATSector...)

		r, err := cfb.Open(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = r.Stream("A")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
