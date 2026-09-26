package ai

import (
	"context"
	"fmt"
	"strings"

	"energyai/internal/analytics"
	"energyai/internal/domain"
	"energyai/internal/format"
)

const SourceTemplate = "TEMPLATE"

type Template struct{}

func (Template) Status(context.Context) Status {
	return Status{Provider: "template", Available: true, Detail: "Explicaciones generadas por reglas a partir de la evidencia"}
}

func (Template) Explain(_ context.Context, f analytics.Finding) (Explanation, error) {
	ev := f.Evidence
	exp := Explanation{Source: SourceTemplate, EvidenceSummary: supportingSignals(ev)}
	switch f.Type {
	case domain.AnomalyReal:
		exp.Reason, exp.RecommendedAction = realAnomaly(f)
	case domain.AnomalyExplainable:
		exp.Reason, exp.RecommendedAction = explainable(ev)
	case domain.AnomalyFalsePositive:
		exp.Reason, exp.RecommendedAction = falsePositive(ev)
	case domain.AnomalyDataQuality:
		exp.Reason, exp.RecommendedAction = dataQuality(ev)
	default:
		return Explanation{}, fmt.Errorf("unknown anomaly type %q", f.Type)
	}
	return exp, nil
}

func supportingSignals(ev domain.Evidence) []string {
	var out []string
	for _, s := range ev.Signals {
		if s.Supports {
			out = append(out, s.Label)
		}
	}
	return out
}

func variable(ev domain.Evidence, name string) (domain.VariableChange, bool) {
	for _, v := range ev.Variables {
		if v.Name == name {
			return v, true
		}
	}
	return domain.VariableChange{}, false
}

func explainingEvent(ev domain.Evidence) *domain.EventEvidence {
	for i := range ev.Events {
		if ev.Events[i].Explains {
			return &ev.Events[i]
		}
	}
	return nil
}

func windowText(w domain.WindowEvidence) string {
	if w.Persistent {
		return fmt.Sprintf("desde el %s (%d h, sigue activo)", format.Date(w.Start), w.Hours)
	}
	return fmt.Sprintf("entre el %s y el %s (%d h)", format.Date(w.Start), format.Date(w.End), w.Hours)
}

func realAnomaly(f analytics.Finding) (string, string) {
	ev := f.Evidence
	var b strings.Builder
	fmt.Fprintf(&b, "Consumo %s frente al baseline %s sin ningún evento operativo que lo explique.",
		format.Pct(ev.DeviationPct, 1), windowText(ev.Window))
	cur, okC := variable(ev, "current_a")
	pf, okP := variable(ev, "power_factor")
	var electrical []string
	if okC && cur.Significant {
		electrical = append(electrical, fmt.Sprintf("la corriente pasó de %s A a %s A", format.Number(cur.Before, 0), format.Number(cur.After, 0)))
	}
	if okP && pf.Significant {
		electrical = append(electrical, fmt.Sprintf("el factor de potencia cayó de %s a %s", format.Number(pf.Before, 2), format.Number(pf.After, 2)))
	}
	if len(electrical) > 0 {
		fmt.Fprintf(&b, " Además %s, lo que confirma un cambio eléctrico real y no un error de medición.", strings.Join(electrical, " y "))
	}
	for _, e := range ev.Events {
		if e.Type == domain.EventUnknown {
			fmt.Fprintf(&b, " El único registro en esa ventana (%s) indica que no se reportó evento operativo.", format.Date(e.Timestamp))
			break
		}
	}

	action := "Revisar la instalación y confirmar con operación si hubo cambios no reportados; monitorear las próximas 24 h."
	if f.Severity == domain.SeverityHigh {
		action = "Priorizar una inspección en sitio del medidor y la instalación en menos de 24 h: identificar cargas nuevas o equipos con falla"
		if okP && pf.Significant {
			action += " (el factor de potencia bajo sugiere carga inductiva o motores defectuosos)"
		}
		action += ", verificar la capacidad del circuito ante el aumento de corriente y contrastar con la operación de la planta."
	}
	return b.String(), action
}

func explainable(ev domain.Evidence) (string, string) {
	e := explainingEvent(ev)
	if e == nil {
		return fmt.Sprintf("Cambio de consumo de %s frente al baseline %s.", format.Pct(ev.DeviationPct, 1), windowText(ev.Window)),
			"Validar con operación la causa del cambio."
	}
	reason := fmt.Sprintf("Cambio de consumo de %s frente al baseline %s, que coincide con el evento operativo «%s» del %s.",
		format.Pct(ev.DeviationPct, 1), windowText(ev.Window), e.Description, format.Date(e.Timestamp))
	if cur, ok := variable(ev, "current_a"); ok && cur.Significant {
		reason += fmt.Sprintf(" La corriente acompaña el cambio (%s A → %s A), consistente con carga adicional real.",
			format.Number(cur.Before, 0), format.Number(cur.After, 0))
	}
	action := "Validar con operación que el nuevo nivel corresponde al cambio reportado; si se confirma, actualizar el baseline del medidor y revisar la potencia contratada y la capacidad de la instalación."
	return reason, action
}

func falsePositive(ev domain.Evidence) (string, string) {
	e := explainingEvent(ev)
	if e == nil {
		return fmt.Sprintf("Cambio de consumo de %s %s explicado por la operación.", format.Pct(ev.DeviationPct, 1), windowText(ev.Window)),
			"No escalar."
	}
	reason := fmt.Sprintf("Caída de consumo de %s %s explicada por el evento «%s» (%s).",
		format.Pct(ev.DeviationPct, 1), windowText(ev.Window), e.Description, format.Date(e.Timestamp))
	if !ev.Window.Persistent {
		reason += " El consumo volvió a su nivel normal al terminar la parada."
	}
	return reason, "No escalar. Cerrar como explicado por mantenimiento programado; no requiere acción adicional."
}

func dataQuality(ev domain.Evidence) (string, string) {
	dq := ev.DataQuality
	v, _ := variable(ev, "voltage_v")
	var parts []string
	if dq.VoltageOutOfBand > 0 {
		parts = append(parts, fmt.Sprintf("%d lecturas con voltaje fuera de rango (entre %s y %s V, nominal %s V)",
			dq.VoltageOutOfBand, format.Number(v.Min, 1), format.Number(v.Max, 1), format.Number(dq.NominalVoltage, 0)))
	}
	if dq.PowerInconsistency > 0 {
		parts = append(parts, fmt.Sprintf("%d lecturas donde la potencia calculada (V·I·FP) no coincide con la energía reportada", dq.PowerInconsistency))
	}
	reason := fmt.Sprintf("Desde el %s el medidor reporta lecturas eléctricas inconsistentes: %s, mientras el consumo se mantiene estable (%s frente al baseline).",
		format.Date(ev.Window.Start), strings.Join(parts, " y "), format.Pct(ev.DeviationPct, 1))
	for _, e := range ev.Events {
		if e.Explains {
			reason += fmt.Sprintf(" Evento reportado: «%s».", e.Description)
			break
		}
	}
	action := fmt.Sprintf("Validar el medidor y su cadena de medición (transformadores de corriente y tensión, conexiones, comunicaciones y firmware); marcar las lecturas desde el %s como no confiables para facturación y análisis hasta corregir.",
		format.Day(ev.Window.Start))
	return reason, action
}
