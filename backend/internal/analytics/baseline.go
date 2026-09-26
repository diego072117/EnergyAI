package analytics

import (
	"errors"
	"fmt"
	"time"

	"energyai/internal/domain"
)

var ErrInsufficientData = errors.New("insufficient data")

type HourStats struct {
	Median float64 `json:"median"`
	MAD    float64 `json:"mad"`
	N      int     `json:"n"`
}

type Baseline struct {
	From       time.Time
	To         time.Time // exclusive
	Days       int
	KWh        [24]HourStats
	Voltage    [24]HourStats
	Current    [24]HourStats
	PF         [24]HourStats
	DailyKWh   float64
	PowerRatio float64 // median of V·I·PF/1000 ÷ kWh
}

// powerRatio relates apparent electrical power to the reported energy. On a
// healthy meter it is stable; a broken relation reveals inconsistent readings.
func powerRatio(r domain.Reading) float64 {
	if r.ConsumptionKWh <= 0 {
		return 0
	}
	return r.VoltageV * r.CurrentA * r.PowerFactor / 1000 / r.ConsumptionKWh
}

func SplitBaseline(clean []domain.Reading, days int) (Baseline, []domain.Reading, error) {
	if len(clean) == 0 {
		return Baseline{}, nil, ErrInsufficientData
	}
	start := clean[0].Timestamp.UTC().Truncate(24 * time.Hour)
	cutoff := start.Add(time.Duration(days) * 24 * time.Hour)

	var ref, analysis []domain.Reading
	for _, r := range clean {
		if r.Timestamp.Before(cutoff) {
			ref = append(ref, r)
		} else {
			analysis = append(analysis, r)
		}
	}
	if len(analysis) == 0 {
		return Baseline{}, nil, fmt.Errorf("%w: no readings after the %d-day baseline", ErrInsufficientData, days)
	}

	b := Baseline{From: start, To: cutoff, Days: days}
	var kwh, volt, cur, pf [24][]float64
	var ratios []float64
	for _, r := range ref {
		h := r.Timestamp.UTC().Hour()
		kwh[h] = append(kwh[h], r.ConsumptionKWh)
		volt[h] = append(volt[h], r.VoltageV)
		cur[h] = append(cur[h], r.CurrentA)
		pf[h] = append(pf[h], r.PowerFactor)
		if pr := powerRatio(r); pr > 0 {
			ratios = append(ratios, pr)
		}
	}
	for h := 0; h < 24; h++ {
		if len(kwh[h]) == 0 {
			return Baseline{}, nil, fmt.Errorf("%w: baseline has no samples for hour %02d", ErrInsufficientData, h)
		}
		b.KWh[h] = hourStats(kwh[h])
		b.Voltage[h] = hourStats(volt[h])
		b.Current[h] = hourStats(cur[h])
		b.PF[h] = hourStats(pf[h])
		b.DailyKWh += b.KWh[h].Median
	}
	b.PowerRatio = median(ratios)
	return b, analysis, nil
}

func hourStats(xs []float64) HourStats {
	m := median(xs)
	return HourStats{Median: m, MAD: mad(xs, m), N: len(xs)}
}

func (b Baseline) ExpectedKWh(t time.Time) float64 { return b.KWh[t.UTC().Hour()].Median }

func (b Baseline) RelDeviation(r domain.Reading) float64 {
	return relChange(b.ExpectedKWh(r.Timestamp), r.ConsumptionKWh)
}
