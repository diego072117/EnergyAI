package analytics

import (
	"math"
	"sort"
)

// madScale converts the median absolute deviation into a standard-deviation
// estimate for normally distributed data.
const madScale = 1.4826

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	m := len(c) / 2
	if len(c)%2 == 1 {
		return c[m]
	}
	return (c[m-1] + c[m]) / 2
}

func mad(xs []float64, med float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	d := make([]float64, len(xs))
	for i, x := range xs {
		d[i] = math.Abs(x - med)
	}
	return median(d)
}

func minMax(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	lo, hi := xs[0], xs[0]
	for _, x := range xs[1:] {
		lo = math.Min(lo, x)
		hi = math.Max(hi, x)
	}
	return lo, hi
}

func clamp(x, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, x))
}

func round(x float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(x*p) / p
}

func relChange(before, after float64) float64 {
	if before <= 0 {
		return 0
	}
	return after/before - 1
}
