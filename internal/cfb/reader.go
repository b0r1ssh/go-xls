package cfb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	headerSize = 512

	// maxRegSector is the maximum valid sector ID.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/9d33df18-7aee-4065-9121-4eabe41c29d4
	maxRegSector = 0xFFFFFFFA

	// maxDIFATEntries is the maximum number of DIFAT entries stored in the header.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/0afa4e43-b18f-432a-9917-4f276eca7a73
	maxDIFATEntries = 109
)

// signature is the expected 8-byte signature at the beginning of a CFB file.
// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/05060311-bfce-4b12-874d-71fd4ce63aea
var signature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

type Reader struct {
	ra io.ReaderAt

	sectorSize     int
	miniSectorSize int

	miniCutoff int64
}

func Open(ra io.ReaderAt) (*Reader, error) {
	hdr := make([]byte, headerSize)
	if _, err := ra.ReadAt(hdr, 0); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if !bytes.Equal(hdr[0:8], signature[:]) {
		return nil, fmt.Errorf("invalid signature, expected %#x, got %#x", signature, hdr[0:8])
	}

	byteOrder := binary.LittleEndian.Uint16(hdr[28:30])
	if byteOrder != 0xFFFE {
		return nil, fmt.Errorf("unsupported byte order mark %#04x", byteOrder)
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
		return nil, fmt.Errorf("unsupported mini sector shift %#04x", miniSectorShift)
	}

	numFATSectors := binary.LittleEndian.Uint32(hdr[44:48])
	difat := make([]uint32, 0, numFATSectors)
	for i := 0; i < maxDIFATEntries && len(difat) < int(numFATSectors); i++ {
		id := binary.LittleEndian.Uint32(hdr[76+i*4 : 80+i*4])
		if isSpecial(id) {
			break
		}
		difat = append(difat, id)
	}

	miniCutoff := binary.LittleEndian.Uint32(hdr[56:60])

	r := &Reader{
		ra:             ra,
		sectorSize:     1 << sectorShift,
		miniSectorSize: 1 << miniSectorShift,
		miniCutoff:     int64(miniCutoff),
	}

	firstDIFATSector := binary.LittleEndian.Uint32(hdr[68:72])
	numDIFATSectors := binary.LittleEndian.Uint32(hdr[72:76])

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

	return &Reader{ra: ra}, nil
}

func (r *Reader) readSector(sector uint32, buf []byte) error {
	if isSpecial(sector) {
		return fmt.Errorf("cannot read reserved sector id %#08x", sector)
	}

	off := (int64(sector) + 1) * int64(r.sectorSize)
	_, err := r.ra.ReadAt(buf, off)
	if err != nil {
		return fmt.Errorf("failed to read sector: %w", err)
	}
	return nil
}

func isSpecial(sector uint32) bool {
	return sector > maxRegSector
}
