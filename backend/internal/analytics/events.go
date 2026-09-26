package analytics

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"time"

	"energyai/internal/domain"
)

var durationRe = regexp.MustCompile(`(?i)(\d+)\s*(hours?|horas?|hrs?|h)\b`)

func eventDuration(desc string) (int, bool) {
	m := durationRe.FindStringSubmatch(desc)
	if m == nil {
		return 0, false
	}
	h, err := strconv.Atoi(m[1])
	return h, err == nil
}

func relatedEvents(events []domain.Event, start, end time.Time, cfg Config) []domain.Event {
	var out []domain.Event
	for _, e := range events {
		off := e.Timestamp.Sub(start)
		inside := !e.Timestamp.Before(start) && !e.Timestamp.After(end)
		if inside || math.Abs(float64(off)) <= float64(cfg.EventWindow) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return absDur(out[i].Timestamp.Sub(start)) < absDur(out[j].Timestamp.Sub(start))
	})
	return out
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// MatchSegmentEvents decides, for each related event, whether it explains the
// consumption change:
//   - SCHEDULED_OUTAGE explains a temporary decrease that recovered,
//   - OPERATIONAL_CHANGE explains a change of level,
//   - DATA_QUALITY and UNKNOWN never explain a consumption change; an UNKNOWN
//     record explicitly confirms that no operational cause was reported.
func MatchSegmentEvents(events []domain.Event, seg Segment, cfg Config) []domain.EventEvidence {
	var out []domain.EventEvidence
	for _, e := range relatedEvents(events, seg.Start, seg.End, cfg) {
		ev := toEvidence(e, seg.Start)
		switch e.Type {
		case domain.EventScheduledOutage:
			if seg.Direction < 0 && !seg.Persistent {
				ev.Explains = true
				ev.Note = "Parada programada alineada con la caída; el consumo se recuperó al terminar."
			} else {
				ev.Note = "Parada programada, pero el cambio no es una caída temporal: no lo explica."
			}
		case domain.EventOperationalChange:
			ev.Explains = true
			ev.Note = "Cambio operativo reportado alineado con el inicio del cambio de consumo."
		case domain.EventDataQuality:
			ev.Note = "Evento de calidad de datos: no explica un cambio real de consumo."
		default:
			ev.Note = "Registro sin evento operativo reportado: no explica el cambio."
		}
		out = append(out, ev)
	}
	return out
}

func MatchQualityEvents(events []domain.Event, start, end time.Time, cfg Config) []domain.EventEvidence {
	var out []domain.EventEvidence
	for _, e := range relatedEvents(events, start, end, cfg) {
		ev := toEvidence(e, start)
		if e.Type == domain.EventDataQuality {
			ev.Explains = true
			ev.Note = "El evento reportado confirma un problema de calidad de datos."
		} else {
			ev.Note = "Evento no relacionado con calidad de datos."
		}
		out = append(out, ev)
	}
	return out
}

func toEvidence(e domain.Event, ref time.Time) domain.EventEvidence {
	return domain.EventEvidence{
		ID:          e.ID,
		Type:        e.Type,
		Timestamp:   e.Timestamp,
		Description: e.Description,
		OffsetHours: round(e.Timestamp.Sub(ref).Hours(), 1),
	}
}

func firstExplaining(evs []domain.EventEvidence) *domain.EventEvidence {
	for i := range evs {
		if evs[i].Explains {
			return &evs[i]
		}
	}
	return nil
}
