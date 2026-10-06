package xls_test

import (
	"os"
	"strings"
	"testing"

	"github.com/b0r1ssh/go-xls"
)

func TestXLSOpenFile(t *testing.T) {
	t.Parallel()

	t.Run("open valid file", func(t *testing.T) {
		t.Parallel()

		_, err := xls.OpenFile("testdata/empty.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
	})

	t.Run("open non-existent file", func(t *testing.T) {
		t.Parallel()

		_, err := xls.OpenFile("testdata/nonexistent.xls")
		if err == nil {
			t.Fatalf("expected error when opening non-existent file")
		}
	})

	t.Run("open invalid file", func(t *testing.T) {
		t.Parallel()

		_, err := xls.OpenFile("testdata/invalid.xls")
		if err == nil {
			t.Fatalf("expected error when opening invalid file")
		}
	})
}

func TestXLSOpenReader(t *testing.T) {
	t.Parallel()

	t.Run("open valid reader", func(t *testing.T) {
		t.Parallel()

		o, err := os.Open("testdata/empty.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		t.Cleanup(func() {
			if err := o.Close(); err != nil {
				t.Fatalf("failed to close reader")
			}
		})

		_, err = xls.OpenReader(o)
		if err != nil {
			t.Fatalf("failed to open reader: %v", err)
		}
	})

	t.Run("open invalid reader", func(t *testing.T) {
		t.Parallel()

		_, err := xls.OpenReader(strings.NewReader(""))
		if err == nil {
			t.Fatalf("expected error when opening invalid reader")
		}
	})
}

func TestXLSSheetNames(t *testing.T) {
	t.Parallel()

	t.Run("sheet names of empty file", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/empty.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}

		names := f.SheetNames()
		if len(names) != 1 {
			t.Fatalf("expected 1 sheet name, got %d", len(names))
		}
	})
}
