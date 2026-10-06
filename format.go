package xls

import (
	"math"
	"strings"
	"time"
)

var (
	epoch1899 = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)
	epoch1904 = time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
)

func isBuiltinDateFormat(ifmt uint16) bool {
	switch {
	case ifmt >= 14 && ifmt <= 22:
		return true
	case ifmt >= 27 && ifmt <= 36:
		return true
	case ifmt >= 45 && ifmt <= 47:
		return true
	}
	return false
}

func isDateFormatCode(code string) bool {
	if i := strings.IndexByte(code, ';'); i >= 0 {
		code = code[:i]
	}

	var b strings.Builder
	for i := 0; i < len(code); i++ {
		switch c := code[i]; c {
		case '\\':
			i++
		case '"':
			i++
			for i < len(code) && code[i] != '"' {
				i++
			}
		case '[':
			for i < len(code) && code[i] != ']' {
				i++
			}
		default:
			b.WriteByte(c)
		}
	}

	stripped := b.String()
	if strings.Contains(stripped, "General") {
		return false
	}

	return strings.ContainsAny(stripped, "yYmMdDhHsS")
}

func (f *File) isDateXFE(ixfe uint16) bool {
	if int(ixfe) >= len(f.dateXFECache) {
		return false
	}

	return f.dateXFECache[ixfe]
}

func isDateXFECode(formats map[uint16]string, ifmt uint16) bool {
	if code, ok := formats[ifmt]; ok {
		return isDateFormatCode(code)
	}

	return isBuiltinDateFormat(ifmt)
}

func (f *File) excelDateTime(serial float64) time.Time {
	epoch := epoch1899
	if f.dateSystem1904 {
		epoch = epoch1904
	}

	days := math.Floor(serial)
	frac := serial - days

	t := epoch.AddDate(0, 0, int(days))
	if frac != 0 {
		t = t.Add(time.Duration(math.Round(frac * 24 * 60 * 60 * float64(time.Second))))
	}
	return t
}
