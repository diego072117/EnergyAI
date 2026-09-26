package format

import (
	"math"
	"strconv"
	"strings"
	"time"
)

var months = [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

func Number(v float64, decimals int) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', decimals, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	if v < 0 && strings.Trim(s, "0.") != "" {
		b.WriteByte('-')
	}
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if frac != "" {
		b.WriteByte(',')
		b.WriteString(frac)
	}
	return b.String()
}

func Pct(v float64, decimals int) string {
	s := Number(v, decimals)
	if !strings.HasPrefix(s, "-") && strings.Trim(s, "0,") != "" {
		s = "+" + s
	}
	return s + "%"
}

func Ratio(v float64) string { return "×" + Number(v, 2) }

func Date(t time.Time) string {
	t = t.UTC()
	return strconv.Itoa(t.Day()) + "-" + months[t.Month()-1] + " " + t.Format("15:04")
}

func Day(t time.Time) string {
	t = t.UTC()
	return strconv.Itoa(t.Day()) + "-" + months[t.Month()-1]
}
