package cfb

import (
	"bytes"
	"fmt"
	"io"
)

const headerSize = 512

// signature is the expected 8-byte signature at the beginning of a CFB file.
// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/05060311-bfce-4b12-874d-71fd4ce63aea
var signature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

type Reader struct {
	ra io.ReaderAt
}

func Open(ra io.ReaderAt) (*Reader, error) {
	hdr := make([]byte, headerSize)
	if _, err := ra.ReadAt(hdr, 0); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if !bytes.Equal(hdr[0:8], signature[:]) {
		return nil, fmt.Errorf("cfb: invalid signature")
	}

	return &Reader{ra: ra}, nil
}
