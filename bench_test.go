package xls_test

import (
	"testing"

	"github.com/b0r1ssh/go-xls"
)

func BenchmarkReadRows(b *testing.B) {
	b.Run("all.xls", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, err := xls.OpenFile("testdata/all.xls")
			if err != nil {
				b.Fatalf("failed to open file: %v", err)
			}

			b.Cleanup(func() {
				err := f.Close()
				if err != nil {
					b.Fatalf("failed to close file: %v", err)
				}
			})

			for _, name := range f.SheetNames() {
				for row := range f.ReadRows(name) {
					if row.Error != nil {
						b.Fatalf("row error: %v", row.Error)
					}
				}
			}
		}
	})
}
