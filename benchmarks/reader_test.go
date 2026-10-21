package benchmarks_test

import (
	"os"
	"testing"

	xls "github.com/b0r1ssh/go-xls"
	extrame "github.com/extrame/xls"
	excel "github.com/jloor/go-excel-reader"
	"github.com/pbnjay/grate"
	_ "github.com/pbnjay/grate/xls" // registers the .xls format with grate.Open
)

const benchmarkFilePath = "../testdata/big.xls"

var (
	benchmarkRows  int
	benchmarkCells int
)

func BenchmarkReadBigXLS(b *testing.B) {
	info, err := os.Stat(benchmarkFilePath)
	if err != nil {
		b.Fatal(err)
	}

	expectedRows, expectedCells, err := countWithGoXLS(benchmarkFilePath)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("b0r1ssh/go-xls", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(info.Size())
		b.ResetTimer()

		for range b.N {
			rows, cells, err := countWithGoXLS(benchmarkFilePath)
			if err != nil {
				b.Fatal(err)
			}
			if rows != expectedRows || cells != expectedCells {
				b.Fatalf("expected %d rows and %d cells, got %d rows and %d cells", expectedRows, expectedCells, rows, cells)
			}
			benchmarkRows, benchmarkCells = rows, cells
		}
	})

	b.Run("extrame/xls", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(info.Size())
		b.ResetTimer()

		for range b.N {
			file, err := os.Open(benchmarkFilePath)
			if err != nil {
				b.Fatal(err)
			}

			workbook, err := extrame.OpenReader(file, "utf-8")
			if err != nil {
				_ = file.Close()
				b.Fatal(err)
			}

			rows, cells := 0, 0
			for i := 0; i < workbook.NumSheets(); i++ {
				sheet := workbook.GetSheet(i)
				if sheet == nil {
					_ = file.Close()
					b.Fatalf("sheet %d is missing", i)
				}
				for rowIndex := 0; rowIndex <= int(sheet.MaxRow); rowIndex++ {
					row := sheet.Row(rowIndex)
					rows++
					for col := row.FirstCol(); col <= row.LastCol(); col++ {
						if row.Col(col) != "" {
							cells++
						}
					}
				}
			}

			if rows != expectedRows || cells != expectedCells {
				_ = file.Close()
				b.Fatalf("expected %d rows and %d cells, got %d rows and %d cells", expectedRows, expectedCells, rows, cells)
			}
			if err := file.Close(); err != nil {
				b.Fatal(err)
			}
			benchmarkRows, benchmarkCells = rows, cells
		}
	})

	b.Run("jloor/go-excel-reader", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(info.Size())
		b.ResetTimer()

		for range b.N {
			reader, err := excel.OpenFile(benchmarkFilePath)
			if err != nil {
				b.Fatal(err)
			}

			rows, cells := 0, 0
			for reader.NextSheet() {
				for reader.Read() {
					rows++
					for col := 0; col < reader.FieldCount(); col++ {
						if reader.GetValue(col) != nil {
							cells++
						}
					}
				}
			}

			if rows != expectedRows || cells != expectedCells {
				_ = reader.Close()
				b.Fatalf("expected %d rows and %d cells, got %d rows and %d cells", expectedRows, expectedCells, rows, cells)
			}
			readErr := reader.Err()
			closeErr := reader.Close()
			if readErr != nil {
				b.Fatal(readErr)
			}
			if closeErr != nil {
				b.Fatal(closeErr)
			}
			benchmarkRows, benchmarkCells = rows, cells
		}
	})

	b.Run("pbnjay/grate", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(info.Size())
		b.ResetTimer()

		for range b.N {
			workbook, err := grate.Open(benchmarkFilePath)
			if err != nil {
				b.Fatal(err)
			}

			sheetNames, err := workbook.List()
			if err != nil {
				_ = workbook.Close()
				b.Fatal(err)
			}

			rows, cells := 0, 0
			for _, name := range sheetNames {
				coll, err := workbook.Get(name)
				if err != nil {
					_ = workbook.Close()
					b.Fatal(err)
				}

				for coll.Next() {
					// grate's DIMENSIONS-based sizing can report a trailing
					// wholly blank row; only count rows that actually hold a
					// value, matching the other readers.
					rowCells := 0
					for _, t := range coll.Types() {
						if t != "blank" {
							rowCells++
						}
					}
					if rowCells > 0 {
						rows++
						cells += rowCells
					}
				}

				if err := coll.Err(); err != nil {
					_ = workbook.Close()
					b.Fatal(err)
				}
			}

			if rows != expectedRows || cells != expectedCells {
				_ = workbook.Close()
				b.Fatalf("expected %d rows and %d cells, got %d rows and %d cells", expectedRows, expectedCells, rows, cells)
			}
			if err := workbook.Close(); err != nil {
				b.Fatal(err)
			}
			benchmarkRows, benchmarkCells = rows, cells
		}
	})
}

func countWithGoXLS(path string) (int, int, error) {
	workbook, err := xls.OpenFile(path)
	if err != nil {
		return 0, 0, err
	}

	rows, cells := 0, 0
	for _, sheetName := range workbook.SheetNames() {
		for row := range workbook.ReadRows(sheetName) {
			if row.Error != nil {
				return 0, 0, row.Error
			}
			rows++
			cells += len(row.Cells)
		}
	}

	return rows, cells, nil
}
