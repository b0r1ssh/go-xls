# Reader benchmarks

[reader_test.go](reader_test.go) compares reading `testdata/big.xls` (15,000
rows, 120,000 non-empty cells, single sheet) with this package and three
other `.xls` readers:

- [`b0r1ssh/go-xls`](https://github.com/b0r1ssh/go-xls) (this package, MIT)
- [`extrame/xls`](https://github.com/extrame/xls) (Apache-2.0)
- [`jloor/go-excel-reader`](https://github.com/jloor/go-excel-reader) (MIT)
- [`pbnjay/grate`](https://github.com/pbnjay/grate) (MIT)

Each benchmark case opens the workbook and reads every row and cell of every
worksheet using that library's own public API, so the numbers reflect
realistic end-to-end usage rather than an internal fast path. Before timing
starts, the harness asserts that every reader counts the same number of rows
and non-empty cells as this package, so the comparison is apples to apples
despite each library modeling cells differently internally.

This harness lives in its own Go module so these benchmark-only dependencies
never affect this package's Go version or dependency set.

## Running

From the repository root:

```sh
cd benchmarks
go test -run '^$' -bench '^BenchmarkReadBigXLS$' -benchmem -benchtime=10x -count=5
```

## Results

Measured on an Apple M2 (`darwin/arm64`), `-benchtime=10x -count=5`; values
are the median of the 5 runs. Results will vary by machine; re-run locally
for your own numbers.

| Reader | Time/op | Throughput | Memory/op | Allocs/op | Relative time |
| --- | ---: | ---: | ---: | ---: | ---: |
| `b0r1ssh/go-xls` | 9.9 ms | 171.4 MB/s | 13.0 MB | 304,182 | 1.0x |
| `pbnjay/grate` | 22.5 ms | 75.5 MB/s | 35.0 MB | 602,687 | 2.3x |
| `jloor/go-excel-reader` | 104.7 ms | 16.2 MB/s | 24.5 MB | 374,050 | 10.6x |
| `extrame/xls` | 376.7 ms | 4.5 MB/s | 25.7 MB | 1,159,802 | 38.1x |

"Relative time" is each reader's time/op divided by this package's time/op
(lower is better; 1.0x is the baseline).
