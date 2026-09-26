package format

import (
	"testing"
	"time"
)

func TestNumber(t *testing.T) {
	cases := []struct {
		v    float64
		d    int
		want string
	}{
		{2208.4, 0, "2.208"},
		{155250, 0, "155.250"},
		{1234567.891, 2, "1.234.567,89"},
		{0.736, 2, "0,74"},
		{-12.5, 1, "-12,5"},
		{-0.001, 1, "0,0"},
		{999, 0, "999"},
	}
	for _, c := range cases {
		if got := Number(c.v, c.d); got != c.want {
			t.Errorf("Number(%v,%d) = %q, want %q", c.v, c.d, got, c.want)
		}
	}
}

func TestPct(t *testing.T) {
	cases := map[float64]string{110.34: "+110,3%", -37.04: "-37,0%", 0: "0,0%"}
	for v, want := range cases {
		if got := Pct(v, 1); got != want {
			t.Errorf("Pct(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestDate(t *testing.T) {
	ts := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	if got := Date(ts); got != "12-sep 14:00" {
		t.Errorf("Date = %q", got)
	}
	if got := Day(ts); got != "12-sep" {
		t.Errorf("Day = %q", got)
	}
}

func TestRatio(t *testing.T) {
	if got := Ratio(2.1); got != "×2,10" {
		t.Errorf("Ratio = %q", got)
	}
}
