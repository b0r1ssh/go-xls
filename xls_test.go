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

	t.Run("open file with malformed BOUNDSHEET record", func(t *testing.T) {
		t.Parallel()

		_, err := xls.OpenFile("testdata/invalid_sheet.xls")
		if err == nil {
			t.Fatalf("expected error when opening file with malformed BOUNDSHEET record")
		}
	})

	t.Run("open file with malformed SST record", func(t *testing.T) {
		t.Parallel()

		tests := []string{
			"invalid_sst_short.xls",
			"invalid_sst_header.xls",
			"invalid_sst_rich_header.xls",
			"invalid_sst_ext_header.xls",
			"invalid_sst_data.xls",
			"invalid_sst_trailer.xls",
		}

		for _, name := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, err := xls.OpenFile("testdata/" + name)
				if err == nil {
					t.Fatalf("expected error when opening file with malformed SST record")
				}
			})
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

	t.Run("sheet name with non-Latin1 characters", func(t *testing.T) {
		t.Parallel()

		f, err := xls.OpenFile("testdata/unicode_sheet.xls")
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}

		names := f.SheetNames()
		if len(names) != 1 || names[0] != "日本語" {
			t.Fatalf("expected sheet name %q, got %v", "日本語", names)
		}
	})
}
