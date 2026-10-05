package cfb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

const (
	headerSize = 512

	// maxRegSector is the maximum valid sector ID.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/9d33df18-7aee-4065-9121-4eabe41c29d4
	maxRegSector = 0xFFFFFFFA

	// sectorFree and sectorEndChain are special sector IDs used in the FAT.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/30e1013a-a0ff-4404-9ccf-d75d835ff404
	sectorFree     = 0xFFFFFFFF
	sectorEndChain = 0xFFFFFFFE

	// maxDIFATEntries is the maximum number of DIFAT entries stored in the header.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/0afa4e43-b18f-432a-9917-4f276eca7a73
	maxDIFATEntries = 109

	// objectTypeRoot represents the root storage object type.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/60fe8611-66c3-496b-b70d-a504c94c9ace
	objectTypeRoot = 0x05
)

// signature is the expected 8-byte signature at the beginning of a CFB file.
// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/05060311-bfce-4b12-874d-71fd4ce63aea
var signature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

type entry struct {
	Name  string
	Type  byte
	Start uint32
	Size  uint64
}

type Reader struct {
	ra io.ReaderAt

	sectorSize     int
	miniSectorSize int

	miniCutoff int64

	fat     []uint32
	miniFAT []uint32

	rootChain []uint32
	dir       []entry
}

func Open(ra io.ReaderAt) (*Reader, error) {
	hdr := make([]byte, headerSize)
	if _, err := ra.ReadAt(hdr, 0); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if !bytes.Equal(hdr[0:8], signature[:]) {
		return nil, fmt.Errorf("invalid signature, expected: %#x, got: %#x", signature, hdr[0:8])
	}

	byteOrder := binary.LittleEndian.Uint16(hdr[28:30])
	if byteOrder != 0xFFFE {
		return nil, fmt.Errorf("unsupported byte order mark: %#04x", byteOrder)
	}

	majorVersion := binary.LittleEndian.Uint16(hdr[26:28])
	sectorShift := binary.LittleEndian.Uint16(hdr[30:32])
	switch {
	case majorVersion == 0x0003 && sectorShift == 0x0009:
	case majorVersion == 0x0004 && sectorShift == 0x000C:
	default:
		return nil, fmt.Errorf("unsupported version/sector shift (version: %#04x, sector shift: %#04x)", majorVersion, sectorShift)
	}

	miniSectorShift := binary.LittleEndian.Uint16(hdr[32:34])
	if miniSectorShift != 0x0006 {
		return nil, fmt.Errorf("unsupported mini sector shift: %#04x", miniSectorShift)
	}

	numFATSectors := binary.LittleEndian.Uint32(hdr[44:48])
	firstDirSector := binary.LittleEndian.Uint32(hdr[48:52])
	miniCutoff := binary.LittleEndian.Uint32(hdr[56:60])
	firstMiniFATSector := binary.LittleEndian.Uint32(hdr[60:64])
	numMiniFATSectors := binary.LittleEndian.Uint32(hdr[64:68])
	firstDIFATSector := binary.LittleEndian.Uint32(hdr[68:72])
	numDIFATSectors := binary.LittleEndian.Uint32(hdr[72:76])

	r := &Reader{
		ra:             ra,
		sectorSize:     1 << sectorShift,
		miniSectorSize: 1 << miniSectorShift,
		miniCutoff:     int64(miniCutoff),
	}

	difat := make([]uint32, 0, numFATSectors)
	for i := 0; i < maxDIFATEntries && len(difat) < int(numFATSectors); i++ {
		id := binary.LittleEndian.Uint32(hdr[76+i*4 : 80+i*4])
		if isSpecial(id) {
			break
		}
		difat = append(difat, id)
	}

	entriesPerSector := r.sectorSize / 4

	buf := make([]byte, r.sectorSize)
	sector := firstDIFATSector
	for i := uint32(0); i < numDIFATSectors && len(difat) < int(numFATSectors); i++ {
		if err := r.readSector(sector, buf); err != nil {
			return nil, err
		}

		before := len(difat)
		for j := range entriesPerSector - 1 {
			id := binary.LittleEndian.Uint32(buf[j*4 : j*4+4])
			if isSpecial(id) {
				continue
			}

			difat = append(difat, id)
		}

		// a self-referential DIFAT chain yields no entries; stop rather than
		if len(difat) == before {
			break
		}

		sector = binary.LittleEndian.Uint32(buf[(entriesPerSector-1)*4:])
		if isSpecial(sector) {
			break
		}
	}

	fat := make([]uint32, 0, len(difat)*entriesPerSector)
	for _, s := range difat {
		if err := r.readSector(s, buf); err != nil {
			return nil, err
		}

		for j := range entriesPerSector {
			fat = append(fat, binary.LittleEndian.Uint32(buf[j*4:j*4+4]))
		}
	}
	r.fat = fat

	dirChain, err := r.chain(firstDirSector, 0)
	if err != nil {
		return nil, err
	}

	entriesPerDirSector := r.sectorSize / 128

	entries := make([]entry, 0, len(dirChain)*entriesPerDirSector)
	dbuf := make([]byte, r.sectorSize)
	for _, s := range dirChain {
		if err := r.readSector(s, dbuf); err != nil {
			return nil, err
		}

		for j := range entriesPerDirSector {
			e := parseEntry(dbuf[j*128:j*128+128], majorVersion == 3)
			if e.Type == 0 { // unallocated slot
				continue
			}

			entries = append(entries, e)
		}
	}
	r.dir = entries

	rootEntry, err := rootEntry(r.dir)
	if err != nil {
		return nil, err
	}

	rootChain, err := r.chain(rootEntry.Start, sectorsFor(int64(rootEntry.Size), r.sectorSize))
	if err != nil {
		return nil, err
	}
	r.rootChain = rootChain

	if numMiniFATSectors > 0 {
		miniChain, err := r.chain(firstMiniFATSector, int(numMiniFATSectors))
		if err != nil {
			return nil, err
		}

		miniFAT := make([]uint32, 0, len(miniChain)*entriesPerSector)
		for _, s := range miniChain {
			if err := r.readSector(s, buf); err != nil {
				return nil, err
			}

			for j := range entriesPerSector {
				miniFAT = append(miniFAT, binary.LittleEndian.Uint32(buf[j*4:j*4+4]))
			}
		}
		r.miniFAT = miniFAT
	}

	return r, nil
}

func (r *Reader) readSector(sector uint32, buf []byte) error {
	if isSpecial(sector) {
		return fmt.Errorf("cannot read reserved sector id: %#08x", sector)
	}

	off := (int64(sector) + 1) * int64(r.sectorSize)
	_, err := r.ra.ReadAt(buf, off)
	if err != nil {
		return fmt.Errorf("failed to read sector: %w", err)
	}
	return nil
}

func (r *Reader) chain(start uint32, hint int) ([]uint32, error) {
	var chain []uint32
	if hint > 0 && hint <= len(r.fat) {
		chain = make([]uint32, 0, hint)
	}

	id := start
	for !isChainEnd(id) {
		if isSpecial(id) {
			return nil, fmt.Errorf("cannot read reserved sector id: %#08x", id)
		}

		if int(id) >= len(r.fat) {
			return nil, errors.New("sector id out of range")
		}

		// every id indexes the FAT, so a chain longer than the FAT repeats one.
		if len(chain) >= len(r.fat) {
			return nil, errors.New("circular sector chain")
		}

		chain = append(chain, id)
		id = r.fat[id]
	}
	return chain, nil
}

func isSpecial(sector uint32) bool {
	return sector > maxRegSector
}

func isChainEnd(sector uint32) bool {
	return sector == sectorEndChain || sector == sectorFree
}

func sectorsFor(size int64, sectorSize int) int {
	if size <= 0 {
		return 0
	}

	n := (size + int64(sectorSize) - 1) / int64(sectorSize)
	return int(n)
}

func parseEntry(b []byte, v3 bool) entry {
	var name string

	nameLen := binary.LittleEndian.Uint16(b[64:66])
	if nameLen >= 2 && nameLen <= 64 {
		u16 := make([]uint16, (nameLen-2)/2)
		for i := range u16 {
			u16[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
		}

		name = string(utf16.Decode(u16))
	}

	size := binary.LittleEndian.Uint64(b[120:128])
	if v3 {
		// the upper 32 bits are undefined in version 3 files.
		size &= 0xFFFFFFFF
	}

	return entry{
		Name:  name,
		Type:  b[66],
		Start: binary.LittleEndian.Uint32(b[116:120]),
		Size:  size,
	}
}

func rootEntry(entries []entry) (entry, error) {
	for i := range entries {
		if entries[i].Type == objectTypeRoot {
			return entries[i], nil
		}
	}
	return entry{}, errors.New("root entry not found")
}
