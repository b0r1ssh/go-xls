package cfb_test

import (
	"bytes"
	"testing"

	"github.com/b0r1sh/go-xls/internal/cfb"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("valid header", func(t *testing.T) {
		t.Parallel()

		validHeader := make([]byte, 512)
		copy(validHeader[0:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})

		_, err := cfb.Open(bytes.NewReader(validHeader))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("invalid header", func(t *testing.T) {
		t.Parallel()

		invalidHeader := make([]byte, 512)
		_, err := cfb.Open(bytes.NewReader(invalidHeader))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("empty header", func(t *testing.T) {
		t.Parallel()

		emptyHeader := make([]byte, 0)
		_, err := cfb.Open(bytes.NewReader(emptyHeader))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
