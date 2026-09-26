package analytics

import (
	"fmt"
	"math"
	"sort"
	"time"

	"energyai/internal/domain"
	"energyai/internal/format"
)

type Finding struct {
	MeterID       string
	Type          domain.AnomalyType
	Severity      domain.Severity
	Confidence    float64
	PriorityScore float64
	WindowStart   time.Time
	WindowEnd     time.Time
	Evidence      domain.Evidence
}

func (f Finding) IsAnomaly() bool { return f.Type != domain.AnomalyFalsePositive }

// Confidence combines three normalised [0,1] scores:
//   - magnitude: how far from normal the behaviour is,
//   - corroboration: share of independent checks backing the classification,
//   - context: how clearly events confirm (or rule out) an explanation.
//
// The result is capped at 0.98: the engine never claims certainty.
func Confidence(magnitude, corroboration, context float64) float64 {
	return round(clamp(0.40+0.25*magnitude+0.25*corroboration+0.10*context, 0, 0.98), 2)
}

var severityWeight = map[domain.Severity]float64{
	domain.SeverityHigh: 60, domain.SeverityMedium: 35, domain.SeverityLow: 10,
}

var typeWeight = map[domain.AnomalyType]float64{
	domain.AnomalyReal: 25, domain.AnomalyDataQuality: 15,
	domain.AnomalyExplainable: 5, domain.AnomalyFalsePositive: 0,
}

func Priority(sev domain.Severity, typ domain.AnomalyType, magnitude float64, recent bool) domain.PriorityBreakdown {
	p := domain.PriorityBreakdown{
		Severity:  severityWeight[sev],
		Type:      typeWeight[typ],
		Magnitude: round(10*clamp(magnitude, 0, 1), 1),
	}
	if recent {
		p.Recency = 5
	}
	p.Total = round(p.Severity+p.Type+p.Magnitude+p.Recency, 1)
	return p
}

func fractionSupported(checks []domain.Signal) float64 {
	if len(checks) == 0 {
		return 0
	}
	n := 0
	for _, c := range checks {
		if c.Supports {
			n++
		}
	}
	return float64(n) / float64(len(checks))
}

func alignment(offsetHours float64, cfg Config) float64 {
	return 1 - 0.5*clamp(math.Abs(offsetHours)/cfg.EventWindow.Hours(), 0, 1)
}

func (m *meterState) findings(cfg Config) []Finding {
	var out []Finding
	if f, ok := m.dataQualityFinding(cfg); ok {
		out = append(out, f)
	}
	for i := range m.segments {
		out = append(out, m.segmentFinding(i, cfg))
	}
	if f, ok := m.spikeFinding(cfg); ok {
		out = append(out, f)
	}
	return out
}

func (m *meterState) baseEvidence(cfg Config) domain.Evidence {
	b := m.baseline
	return domain.Evidence{
		Baseline: domain.BaselineEvidence{From: b.From, To: b.To, Days: b.Days, DailyKWh: round(b.DailyKWh, 1)},
		Current: domain.CurrentEvidence{
			ReferenceTime: m.stats.PeriodEnd,
			Last24hKWh:    round(m.stats.Last24hKWh, 1),
			VariationPct:  round(m.stats.VariationPct, 1),
		},
		DataQuality: domain.DataQualityEvidence{
			NominalVoltage:     cfg.NominalVoltage,
			VoltageOutOfBand:   m.dq.VoltageOutOfBand,
			PowerInconsistency: m.dq.PowerInconsistency,
			VoltageJumps:       m.dq.VoltageJumps,
			MissingHours:       m.validation.MissingHours,
			Duplicates:         m.validation.Duplicates,
			InvalidValues:      m.validation.InvalidValues,
		},
	}
}

func (m *meterState) recent(end time.Time) bool {
	return !end.Before(m.stats.PeriodEnd.Add(-24 * time.Hour))
}

func (m *meterState) dqAffectedIn(from, to int) int {
	n := 0
	for _, i := range m.dq.Affected {
		if i >= from && i <= to {
			n++
		}
	}
	return n
}

func variables(e Electrical, cfg Config) []domain.VariableChange {
	band := cfg.NominalVoltage * cfg.VoltageTolerance
	kwhPct := relChange(e.KWhBefore, e.KWhAfter) * 100
	curPct := relChange(e.CurrentBefore, e.CurrentAfter) * 100
	return []domain.VariableChange{
		{
			Name: "consumption_kwh", Label: "Consumo", Unit: "kWh/h",
			Before: round(e.KWhBefore, 2), After: round(e.KWhAfter, 2),
			Min: round(e.KWhMin, 2), Max: round(e.KWhMax, 2),
			ChangePct: round(kwhPct, 1), ChangeAbs: round(e.KWhAfter-e.KWhBefore, 2),
			Significant: math.Abs(kwhPct) >= cfg.ShiftThreshold*100,
		},
		{
			Name: "current_a", Label: "Corriente", Unit: "A",
			Before: round(e.CurrentBefore, 1), After: round(e.CurrentAfter, 1),
			Min: round(e.CurrentMin, 1), Max: round(e.CurrentMax, 1),
			ChangePct: round(curPct, 1), ChangeAbs: round(e.CurrentAfter-e.CurrentBefore, 1),
			Significant: math.Abs(curPct) >= 10,
		},
		{
			Name: "voltage_v", Label: "Voltaje", Unit: "V",
			Before: round(e.VoltageBefore, 1), After: round(e.VoltageAfter, 1),
			Min: round(e.VoltageMin, 1), Max: round(e.VoltageMax, 1),
			ChangePct: round(e.VoltageChangePct, 1), ChangeAbs: round(e.VoltageAfter-e.VoltageBefore, 1),
			Significant: math.Abs(e.VoltageChangePct) >= cfg.VoltageChangePct ||
				math.Abs(e.VoltageMin-cfg.NominalVoltage) > band || math.Abs(e.VoltageMax-cfg.NominalVoltage) > band,
		},
		{
			Name: "power_factor", Label: "Factor de potencia", Unit: "",
			Before: round(e.PFBefore, 3), After: round(e.PFAfter, 3),
			Min: round(e.PFMin, 3), Max: round(e.PFMax, 3),
			ChangePct: round(relChange(e.PFBefore, e.PFAfter)*100, 1), ChangeAbs: round(e.PFAfter-e.PFBefore, 3),
			Significant: math.Abs(e.PFDrop) >= cfg.PFDropThreshold || e.PFMin < e.PFBefore-0.2,
		},
	}
}

func direction(dev float64) domain.Direction {
	switch {
	case dev > 0:
		return domain.DirectionUp
	case dev < 0:
		return domain.DirectionDown
	}
	return domain.DirectionNone
}

func coMoveSignal(e Electrical) domain.Signal {
	return domain.Signal{
		Code:     "CURRENT_COMOVES",
		Label:    fmt.Sprintf("La corriente acompaña el cambio de consumo (%s corriente vs %s consumo)", format.Ratio(e.CurrentRatio), format.Ratio(e.KWhRatio)),
		Supports: e.CurrentCoMoves,
	}
}

func noDQSignal(affected int) domain.Signal {
	label := "Sin problemas de calidad de datos en la ventana"
	if affected > 0 {
		label = fmt.Sprintf("%d lecturas con problemas de calidad en la ventana", affected)
	}
	return domain.Signal{Code: "NO_DATA_QUALITY", Label: label, Supports: affected == 0}
}

func alignedSignal(ev *domain.EventEvidence, cfg Config) domain.Signal {
	return domain.Signal{
		Code:     "EVENT_ALIGNED",
		Label:    fmt.Sprintf("Evento %s a %s h del inicio del cambio", ev.Type, format.Number(math.Abs(ev.OffsetHours), 0)),
		Supports: math.Abs(ev.OffsetHours) <= cfg.AlignedEventHours,
	}
}

func (m *meterState) segmentFinding(i int, cfg Config) Finding {
	seg, el, evs := m.segments[i], m.segElec[i], m.segEvents[i]
	absDev := math.Abs(seg.MeanDev)
	exp := firstExplaining(evs)
	noDQ := noDQSignal(m.dqAffectedIn(seg.StartIdx, seg.EndIdx))

	typ := domain.AnomalyReal
	if exp != nil {
		typ = domain.AnomalyExplainable
		if exp.Type == domain.EventScheduledOutage {
			typ = domain.AnomalyFalsePositive
		}
	}

	shift := domain.Signal{
		Code:     "LEVEL_SHIFT",
		Label:    fmt.Sprintf("Consumo %s frente al baseline durante %d h desde el %s", format.Pct(seg.MeanDev*100, 1), seg.Hours, format.Date(seg.Start)),
		Supports: true,
	}
	sustained := seg.Persistent || seg.Hours >= 24
	var checks []domain.Signal
	var extra []domain.Signal
	var ctx float64
	var sev domain.Severity

	switch typ {
	case domain.AnomalyReal:
		checks = []domain.Signal{
			{Code: "PERSISTENT", Label: fmt.Sprintf("Cambio sostenido durante %d h", seg.Hours), Supports: sustained},
			coMoveSignal(el),
			{
				Code: "ELECTRICAL_CHANGE",
				Label: fmt.Sprintf("Cambio eléctrico: FP %s → %s, voltaje %s",
					format.Number(el.PFBefore, 2), format.Number(el.PFAfter, 2), format.Pct(el.VoltageChangePct, 1)),
				Supports: el.ElectricalChange(cfg),
			},
			noDQ,
		}
		extra = []domain.Signal{{Code: "NO_EXPLAINING_EVENT", Label: "Ningún evento operativo explica el cambio", Supports: true}}
		ctx = 0.8
		if len(evs) > 0 { // an explicit record confirms no operational cause
			ctx = 1
		}
		corroborating := 0
		if el.CurrentCoMoves {
			corroborating++
		}
		if el.ElectricalChange(cfg) {
			corroborating++
		}
		sev = domain.SeverityMedium
		if absDev >= 0.5 || (absDev >= cfg.ShiftThreshold && corroborating >= 2) {
			sev = domain.SeverityHigh
		}
	case domain.AnomalyExplainable:
		checks = []domain.Signal{
			alignedSignal(exp, cfg),
			{Code: "NEW_STABLE_LEVEL", Label: "Nuevo nivel de consumo sostenido tras el evento", Supports: sustained},
			coMoveSignal(el),
			noDQ,
		}
		ctx = alignment(exp.OffsetHours, cfg)
		sev = domain.SeverityLow
		if absDev >= cfg.ShiftThreshold {
			sev = domain.SeverityMedium
		}
	case domain.AnomalyFalsePositive:
		dur, ok := eventDuration(exp.Description)
		durLabel := fmt.Sprintf("Duración observada %d h (el evento no indica duración)", seg.Hours)
		if ok {
			durLabel = fmt.Sprintf("Duración coincide: %d h reportadas vs %d h observadas", dur, seg.Hours)
		}
		checks = []domain.Signal{
			alignedSignal(exp, cfg),
			{Code: "RECOVERED", Label: "El consumo volvió a su nivel normal", Supports: !seg.Persistent},
			{Code: "DURATION_MATCH", Label: durLabel, Supports: ok && absInt(dur-seg.Hours) <= 3},
			coMoveSignal(el),
		}
		ctx = alignment(exp.OffsetHours, cfg)
		sev = domain.SeverityLow
	}

	ev := m.baseEvidence(cfg)
	ev.Window = domain.WindowEvidence{Start: seg.Start, End: seg.End, Hours: seg.Hours, Persistent: seg.Persistent}
	ev.Direction = direction(seg.MeanDev)
	ev.DeviationPct = round(seg.MeanDev*100, 1)
	ev.Variables = variables(el, cfg)
	ev.Signals = append(append([]domain.Signal{shift}, checks...), extra...)
	ev.Events = evs
	return m.finish(typ, sev, ev, clamp(absDev, 0, 1), fractionSupported(checks), ctx, absDev/1.5)
}

func (m *meterState) dataQualityFinding(cfg Config) (Finding, bool) {
	dq := m.dq
	if dq.Signals < cfg.DQMinSignals || dq.FirstIdx < 0 {
		return Finding{}, false
	}
	// Electrical anomalies inside a coherent consumption change are physics,
	// not bad data: they are reported by the segment finding instead.
	for i, seg := range m.segments {
		if seg.Contains(dq.FirstIdx) && m.segElec[i].CurrentCoMoves {
			return Finding{}, false
		}
	}
	an := m.analysis
	last := len(an) - 1
	start, end := an[dq.FirstIdx].Timestamp, an[dq.LastIdx].Timestamp
	el := AnalyzeElectrical(an, dq.FirstIdx, last, m.baseline, cfg)
	stable := true
	for _, seg := range m.segments {
		if seg.EndIdx >= dq.FirstIdx {
			stable = false
		}
	}
	evs := MatchQualityEvents(m.in.Events, start, end, cfg)
	reported := firstExplaining(evs) != nil
	minOcc := cfg.DQMinOccurrences
	band := cfg.NominalVoltage * cfg.VoltageTolerance

	checks := []domain.Signal{
		{
			Code: "VOLTAGE_OUT_OF_BAND",
			Label: fmt.Sprintf("%d lecturas con voltaje fuera de %s V ±%s%% (rango %s–%s V)",
				dq.VoltageOutOfBand, format.Number(cfg.NominalVoltage, 0), format.Number(cfg.VoltageTolerance*100, 0),
				format.Number(el.VoltageMin, 1), format.Number(el.VoltageMax, 1)),
			Supports: dq.VoltageOutOfBand >= minOcc,
		},
		{
			Code:     "POWER_INCONSISTENCY",
			Label:    fmt.Sprintf("%d lecturas donde V·I·FP no es coherente con la energía reportada", dq.PowerInconsistency),
			Supports: dq.PowerInconsistency >= minOcc,
		},
		{
			Code:     "VOLTAGE_JUMPS",
			Label:    fmt.Sprintf("%d saltos bruscos de voltaje (> %s V) entre lecturas consecutivas", dq.VoltageJumps, format.Number(band, 0)),
			Supports: dq.VoltageJumps >= minOcc,
		},
		{
			Code:     "CONSUMPTION_STABLE",
			Label:    fmt.Sprintf("El consumo se mantiene estable (%s frente al baseline)", format.Pct((el.KWhRatio-1)*100, 1)),
			Supports: stable,
		},
	}
	extra := domain.Signal{Code: "DQ_EVENT_REPORTED", Label: "Evento de calidad de datos reportado", Supports: reported}
	ctx := 0.7
	if reported {
		ctx = 1
	}
	sev := domain.SeverityMedium
	if dq.AffectedFraction >= 0.10 || dq.VoltageOutOfBand >= minOcc {
		sev = domain.SeverityHigh
	}

	ev := m.baseEvidence(cfg)
	ev.Window = domain.WindowEvidence{
		Start: start, End: end,
		Hours:      int(end.Sub(start)/time.Hour) + 1,
		Persistent: last-dq.LastIdx < 24,
	}
	ev.Direction = domain.DirectionNone
	ev.DeviationPct = round((el.KWhRatio-1)*100, 1)
	ev.Variables = variables(el, cfg)
	ev.Signals = append(checks, extra)
	ev.Events = evs
	ev.DataQuality.AffectedReadings = len(dq.Affected)
	ev.DataQuality.ReadingsInWindow = len(an) - dq.FirstIdx
	ev.DataQuality.AffectedPct = round(dq.AffectedFraction*100, 1)
	mag := clamp(dq.AffectedFraction*2, 0, 1)
	return m.finish(domain.AnomalyDataQuality, sev, ev, mag, fractionSupported(checks), ctx, mag), true
}

func (m *meterState) spikeFinding(cfg Config) (Finding, bool) {
	if len(m.spikes) == 0 {
		return Finding{}, false
	}
	top := m.spikes[0]
	for _, s := range m.spikes[1:] {
		if math.Abs(s.Dev) > math.Abs(top.Dev) {
			top = s
		}
	}
	first, last := m.spikes[0], m.spikes[len(m.spikes)-1]
	dir := 1
	if top.Dev < 0 {
		dir = -1
	}
	pseudo := Segment{Start: first.Timestamp, End: last.Timestamp, Direction: dir}
	evs := MatchSegmentEvents(m.in.Events, pseudo, cfg)
	exp := firstExplaining(evs)
	typ, sev := domain.AnomalyReal, domain.SeverityLow
	if math.Abs(top.Dev) >= 0.5 {
		sev = domain.SeverityMedium
	}
	ctx := 0.8
	if exp != nil {
		typ, sev = domain.AnomalyExplainable, domain.SeverityLow
		ctx = alignment(exp.OffsetHours, cfg)
	}
	el := AnalyzeElectrical(m.analysis, top.Idx, top.Idx, m.baseline, cfg)
	checks := []domain.Signal{
		{Code: "ISOLATED_OUTLIERS", Label: fmt.Sprintf("%d lecturas aisladas fuera de su rango horario", len(m.spikes)), Supports: true},
		{Code: "ROBUST_Z", Label: fmt.Sprintf("z robusto máximo %s con desviación %s", format.Number(top.Z, 1), format.Pct(top.Dev*100, 1)), Supports: true},
		noDQSignal(m.dqAffectedIn(first.Idx, last.Idx)),
	}
	ev := m.baseEvidence(cfg)
	ev.Window = domain.WindowEvidence{Start: first.Timestamp, End: last.Timestamp, Hours: int(last.Timestamp.Sub(first.Timestamp)/time.Hour) + 1}
	ev.Direction = direction(top.Dev)
	ev.DeviationPct = round(top.Dev*100, 1)
	ev.Variables = variables(el, cfg)
	ev.Signals = checks
	ev.Events = evs
	return m.finish(typ, sev, ev, clamp(math.Abs(top.Dev), 0, 1), fractionSupported(checks), ctx, math.Abs(top.Dev)/1.5), true
}

func (m *meterState) finish(typ domain.AnomalyType, sev domain.Severity, ev domain.Evidence, mag, corr, ctx, prioMag float64) Finding {
	conf := Confidence(mag, corr, ctx)
	ev.Confidence = domain.ConfidenceBreakdown{Magnitude: round(mag, 2), Corroboration: round(corr, 2), Context: round(ctx, 2), Value: conf}
	ev.Priority = Priority(sev, typ, prioMag, m.recent(ev.Window.End))
	return Finding{
		MeterID:       m.in.MeterID,
		Type:          typ,
		Severity:      sev,
		Confidence:    conf,
		PriorityScore: ev.Priority.Total,
		WindowStart:   ev.Window.Start,
		WindowEnd:     ev.Window.End,
		Evidence:      ev,
	}
}

func SortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].PriorityScore != fs[j].PriorityScore {
			return fs[i].PriorityScore > fs[j].PriorityScore
		}
		if fs[i].Confidence != fs[j].Confidence {
			return fs[i].Confidence > fs[j].Confidence
		}
		return fs[i].MeterID < fs[j].MeterID
	})
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
