package analytics

import (
	"math"
	"time"

	"energyai/internal/domain"
)

type Segment struct {
	StartIdx   int
	EndIdx     int // inclusive
	Start      time.Time
	End        time.Time
	Direction  int     // +1 increase, -1 decrease
	MeanDev    float64 // mean signed relative deviation inside the segment
	Hours      int
	Persistent bool // still active at the last reading
}

func (s Segment) Contains(i int) bool { return i >= s.StartIdx && i <= s.EndIdx }

// DetectLevelShifts finds sustained changes (persistent shifts, outages): the
// trailing rolling mean of the hourly relative deviation must stay beyond
// ±ShiftThreshold for at least ShiftMinHours consecutive hours. The rolling
// mean lags, so segment boundaries are then refined to the first and last
// individual reading beyond the threshold.
func DetectLevelShifts(an []domain.Reading, b Baseline, cfg Config) []Segment {
	n := len(an)
	if n == 0 {
		return nil
	}
	rel := make([]float64, n)
	for i, r := range an {
		rel[i] = b.RelDeviation(r)
	}

	w := max(cfg.ShiftWindowHours, 1)
	flag := make([]int, n)
	sum := 0.0
	for k := 0; k < n; k++ {
		sum += rel[k]
		if k >= w {
			sum -= rel[k-w]
		}
		mv := sum / float64(min(k+1, w))
		switch {
		case mv > cfg.ShiftThreshold:
			flag[k] = 1
		case mv < -cfg.ShiftThreshold:
			flag[k] = -1
		}
	}

	var segs []Segment
	for k := 0; k < n; {
		if flag[k] == 0 {
			k++
			continue
		}
		dir, j := flag[k], k
		for j+1 < n && flag[j+1] == dir {
			j++
		}
		if j-k+1 >= cfg.ShiftMinHours {
			if seg, ok := refineSegment(an, rel, k, j, dir, w, cfg.ShiftThreshold); ok {
				segs = append(segs, seg)
			}
		}
		k = j + 1
	}
	return segs
}

func refineSegment(an []domain.Reading, rel []float64, runStart, runEnd, dir, w int, thr float64) (Segment, bool) {
	beyond := func(i int) bool { return rel[i]*float64(dir) > thr }
	start := -1
	for i := max(0, runStart-w+1); i <= runEnd; i++ {
		if beyond(i) {
			start = i
			break
		}
	}
	if start < 0 {
		return Segment{}, false
	}
	end := start
	for i := runEnd; i >= start; i-- {
		if beyond(i) {
			end = i
			break
		}
	}
	devs := rel[start : end+1]
	n := len(an)
	return Segment{
		StartIdx:   start,
		EndIdx:     end,
		Start:      an[start].Timestamp,
		End:        an[end].Timestamp,
		Direction:  dir,
		MeanDev:    mean(devs),
		Hours:      int(an[end].Timestamp.Sub(an[start].Timestamp)/time.Hour) + 1,
		Persistent: runEnd == n-1 && n-1-end < w,
	}, true
}

type Spike struct {
	Idx       int
	Timestamp time.Time
	Dev       float64
	Z         float64
}

// DetectSpikes flags isolated outliers outside the level-shift segments. Both
// the robust z-score and the relative deviation must be exceeded: with only a
// week of history the MAD can be tiny and the z-score alone would flag normal
// ±15% noise.
func DetectSpikes(an []domain.Reading, b Baseline, segs []Segment, cfg Config) []Spike {
	var out []Spike
	for i, r := range an {
		if inAnySegment(segs, i) {
			continue
		}
		hs := b.KWh[r.Timestamp.UTC().Hour()]
		if hs.Median <= 0 {
			continue
		}
		dev := relChange(hs.Median, r.ConsumptionKWh)
		z := math.Inf(1)
		if hs.MAD > 0 {
			z = math.Abs(r.ConsumptionKWh-hs.Median) / (madScale * hs.MAD)
		}
		if z > cfg.SpikeZ && math.Abs(dev) > cfg.SpikeMinDev {
			out = append(out, Spike{Idx: i, Timestamp: r.Timestamp, Dev: dev, Z: z})
		}
	}
	return out
}

func inAnySegment(segs []Segment, i int) bool {
	for _, s := range segs {
		if s.Contains(i) {
			return true
		}
	}
	return false
}

type DQResult struct {
	VoltageOutOfBand   int
	PowerInconsistency int
	VoltageJumps       int
	Affected           []int // indices with out-of-band voltage or inconsistent power
	FirstIdx           int   // -1 when nothing was flagged
	LastIdx            int
	Signals            int
	AffectedFraction   float64 // affected / readings from the first issue on
}

// DetectDataQuality checks the physical consistency of each reading:
//   - voltage outside nominal ± tolerance,
//   - V·I·PF/1000 ÷ kWh far from the meter's own baseline ratio (the electrical
//     variables do not explain the energy reported),
//   - abrupt voltage jumps between consecutive readings.
//
// A check becomes a "signal" when it occurs at least DQMinOccurrences times.
func DetectDataQuality(an []domain.Reading, b Baseline, v ValidationReport, cfg Config) DQResult {
	res := DQResult{FirstIdx: -1, LastIdx: -1}
	band := cfg.NominalVoltage * cfg.VoltageTolerance
	for i, r := range an {
		affected := false
		if math.Abs(r.VoltageV-cfg.NominalVoltage) > band {
			res.VoltageOutOfBand++
			affected = true
		}
		if b.PowerRatio > 0 {
			if pr := powerRatio(r); pr > 0 && math.Abs(pr/b.PowerRatio-1) > cfg.PowerRatioTolerance {
				res.PowerInconsistency++
				affected = true
			}
		}
		if i > 0 && math.Abs(r.VoltageV-an[i-1].VoltageV) > band {
			res.VoltageJumps++
		}
		if affected {
			res.Affected = append(res.Affected, i)
			if res.FirstIdx < 0 {
				res.FirstIdx = i
			}
			res.LastIdx = i
		}
	}
	for _, c := range []int{res.VoltageOutOfBand, res.PowerInconsistency, res.VoltageJumps,
		v.MissingHours + v.Duplicates + v.InvalidValues} {
		if c >= cfg.DQMinOccurrences {
			res.Signals++
		}
	}
	if res.FirstIdx >= 0 {
		res.AffectedFraction = float64(len(res.Affected)) / float64(len(an)-res.FirstIdx)
	}
	return res
}

type Electrical struct {
	KWhBefore, KWhAfter         float64
	CurrentBefore, CurrentAfter float64
	VoltageBefore, VoltageAfter float64
	PFBefore, PFAfter           float64
	KWhMin, KWhMax              float64
	CurrentMin, CurrentMax      float64
	VoltageMin, VoltageMax      float64
	PFMin, PFMax                float64
	KWhRatio, CurrentRatio      float64
	CurrentCoMoves              bool    // current changes in proportion to energy
	PFDrop                      float64 // PFBefore - PFAfter
	VoltageChangePct            float64
}

func AnalyzeElectrical(an []domain.Reading, from, to int, b Baseline, cfg Config) Electrical {
	var ek, ei, ev, ep, ak, ai, av, ap []float64
	for _, r := range an[from : to+1] {
		h := r.Timestamp.UTC().Hour()
		ek = append(ek, b.KWh[h].Median)
		ei = append(ei, b.Current[h].Median)
		ev = append(ev, b.Voltage[h].Median)
		ep = append(ep, b.PF[h].Median)
		ak = append(ak, r.ConsumptionKWh)
		ai = append(ai, r.CurrentA)
		av = append(av, r.VoltageV)
		ap = append(ap, r.PowerFactor)
	}
	e := Electrical{
		KWhBefore: mean(ek), KWhAfter: mean(ak),
		CurrentBefore: mean(ei), CurrentAfter: mean(ai),
		VoltageBefore: mean(ev), VoltageAfter: mean(av),
		PFBefore: mean(ep), PFAfter: mean(ap),
	}
	e.KWhMin, e.KWhMax = minMax(ak)
	e.CurrentMin, e.CurrentMax = minMax(ai)
	e.VoltageMin, e.VoltageMax = minMax(av)
	e.PFMin, e.PFMax = minMax(ap)
	e.KWhRatio = safeRatio(e.KWhAfter, e.KWhBefore)
	e.CurrentRatio = safeRatio(e.CurrentAfter, e.CurrentBefore)
	e.PFDrop = e.PFBefore - e.PFAfter
	e.VoltageChangePct = relChange(e.VoltageBefore, e.VoltageAfter) * 100
	sameSide := (e.KWhRatio-1)*(e.CurrentRatio-1) > 0
	e.CurrentCoMoves = sameSide && e.KWhRatio > 0 &&
		math.Abs(e.CurrentRatio-e.KWhRatio)/e.KWhRatio <= cfg.CoMoveTolerance
	return e
}

func (e Electrical) ElectricalChange(cfg Config) bool {
	return e.PFDrop >= cfg.PFDropThreshold || math.Abs(e.VoltageChangePct) >= cfg.VoltageChangePct
}

func safeRatio(a, b float64) float64 {
	if b <= 0 {
		return 0
	}
	return a / b
}
