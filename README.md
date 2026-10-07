# go-xls

[![CI](https://github.com/b0r1ssh/go-xls/actions/workflows/go-test.yml/badge.svg)](https://github.com/b0r1ssh/go-xls/actions/workflows/go-test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/b0r1ssh/go-xls.svg)](https://pkg.go.dev/github.com/b0r1ssh/go-xls)
[![codecov](https://codecov.io/gh/b0r1ssh/go-xls/graph/badge.svg?token=4PCHLI2CGJ)](https://codecov.io/gh/b0r1ssh/go-xls)

A small Go package for reading legacy Microsoft Excel `.xls` (BIFF8)
workbooks. It lists worksheet names and streams each sheet's rows as an
iterator without loading the whole sheet into memory.

## Features

- Open workbooks from a file path or any `io.ReaderAt`
- List the names of a workbook's visible worksheets
- Stream rows of a sheet as an `iter.Seq[Row]` iterator as they are parsed,
  instead of buffering the whole sheet
- String, numeric, date, and boolean cell values, with spreadsheet-style
  column labels (`A`, `B`, ..., `AA`, ...)
- Each `Row` reports `FirstCol`/`LastCol`, the 0-based column range Excel
  recorded as having data, even if some of those columns are empty

## Installation

```sh
go get github.com/b0r1ssh/go-xls
```

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/b0r1ssh/go-xls"
)

func main() {
	f, err := xls.OpenFile("report.xls")
	if err != nil {
		log.Fatal(err)
	}

	for _, name := range f.SheetNames() {
		fmt.Println("sheet:", name)
	}

	for row := range f.ReadRows(f.SheetNames()[0]) {
		if row.Error != nil {
			log.Fatal(row.Error)
		}

		for _, cell := range row.Cells {
			fmt.Printf("%s%d = %v\n", cell.Column, row.Index+1, cell.Value)
		}
	}
}
```

`Row.Cells` only contains cells that hold a value, so blank cells in the
middle of a row are skipped. Use `Row.FirstCol` and `Row.LastCol` to know the
full column range Excel recorded for the row instead of inferring it from
`Cells`:

```go
for row := range f.ReadRows("Sheet1") {
	fmt.Printf("row %d spans columns %d to %d\n", row.Index, row.FirstCol, row.LastCol)
}
```

## Release workflow for contributors

Merging a pull request into `main` automatically creates a Git tag and a
GitHub release. Before merging, use at most one of these labels to select the
semantic version increment:

- `patch`: bug fixes and other backward-compatible changes (`v1.2.3` to
	`v1.2.4`).
- `minor`: new backward-compatible functionality (`v1.2.3` to `v1.3.0`).
- `major`: breaking API changes (`v1.2.3` to `v2.0.0`).

When no release label is present, the workflow uses `patch`. Do not combine
`patch`, `minor`, and `major` labels on one pull request; the release workflow
will fail if more than one is present.

Use the `skip-release` label for documentation, CI, or other changes that do
not need a new module version. When a pull request with this label is merged,
the publish workflow skips the release entirely.