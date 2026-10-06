package xls

import (
	"math"
	"strings"
	"time"
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
	if int(ixfe) >= len(f.xfFormats) {
		return false
	}

	ifmt := f.xfFormats[ixfe]
	if code, ok := f.formats[ifmt]; ok {
		return isDateFormatCode(code)
	}

	return isBuiltinDateFormat(ifmt)
}

func (f *File) excelDateTime(serial float64) time.Time {
	epoch := time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)
	if f.dateSystem1904 {
		epoch = time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	days := math.Floor(serial)
	frac := serial - days

	t := epoch.AddDate(0, 0, int(days))
	if frac != 0 {
		t = t.Add(time.Duration(math.Round(frac * 24 * 60 * 60 * float64(time.Second))))
	}
	return t
}
