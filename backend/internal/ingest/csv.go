package ingest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"energyai/internal/domain"
)

var timeLayouts = []string{"2006-01-02 15:04:05", "2006-01-02 15:04", time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04"}

// ParseTime accepts the timestamp formats found in the source files. Values
// without a zone are interpreted as UTC.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", s)
}

type table struct {
	r    *csv.Reader
	cols map[string]int
	line int
}

func newTable(r io.Reader, required ...string) (*table, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	cols := map[string]int{}
	for i, h := range header {
		cols[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")))] = i
	}
	for _, c := range required {
		if _, ok := cols[c]; !ok {
			return nil, fmt.Errorf("missing column %q", c)
		}
	}
	return &table{r: cr, cols: cols, line: 1}, nil
}

func (t *table) next() ([]string, error) {
	t.line++
	return t.r.Read()
}

func (t *table) get(rec []string, col string) string {
	if i, ok := t.cols[col]; ok && i < len(rec) {
		return strings.TrimSpace(rec[i])
	}
	return ""
}

func (t *table) float(rec []string, col string) (float64, error) {
	v, err := strconv.ParseFloat(t.get(rec, col), 64)
	if err != nil {
		return 0, fmt.Errorf("line %d: column %s: %w", t.line, col, err)
	}
	return v, nil
}

func ParseReadings(r io.Reader) ([]domain.Reading, error) {
	t, err := newTable(r, "meter_id", "timestamp", "consumption_kwh", "voltage_v", "current_a", "power_factor")
	if err != nil {
		return nil, err
	}
	var out []domain.Reading
	for {
		rec, err := t.next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", t.line, err)
		}
		rd := domain.Reading{MeterID: t.get(rec, "meter_id"), Status: t.get(rec, "status")}
		if rd.MeterID == "" {
			return nil, fmt.Errorf("line %d: empty meter_id", t.line)
		}
		if rd.Timestamp, err = ParseTime(t.get(rec, "timestamp")); err != nil {
			return nil, fmt.Errorf("line %d: %w", t.line, err)
		}
		if rd.ConsumptionKWh, err = t.float(rec, "consumption_kwh"); err != nil {
			return nil, err
		}
		if rd.VoltageV, err = t.float(rec, "voltage_v"); err != nil {
			return nil, err
		}
		if rd.CurrentA, err = t.float(rec, "current_a"); err != nil {
			return nil, err
		}
		if rd.PowerFactor, err = t.float(rec, "power_factor"); err != nil {
			return nil, err
		}
		if rd.Status == "" {
			rd.Status = "OK"
		}
		out = append(out, rd)
	}
}

func ParseEvents(r io.Reader) ([]domain.Event, error) {
	t, err := newTable(r, "meter_id", "event_timestamp", "event_type")
	if err != nil {
		return nil, err
	}
	var out []domain.Event
	for {
		rec, err := t.next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", t.line, err)
		}
		ev := domain.Event{
			MeterID:     t.get(rec, "meter_id"),
			Type:        domain.EventType(strings.ToUpper(t.get(rec, "event_type"))),
			Description: t.get(rec, "description"),
		}
		if ev.MeterID == "" {
			return nil, fmt.Errorf("line %d: empty meter_id", t.line)
		}
		if ev.Timestamp, err = ParseTime(t.get(rec, "event_timestamp")); err != nil {
			return nil, fmt.Errorf("line %d: %w", t.line, err)
		}
		if ev.Type == "" {
			ev.Type = domain.EventUnknown
		}
		out = append(out, ev)
	}
}

func ReadReadingsFile(path string) ([]domain.Reading, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseReadings(f)
}

func ReadEventsFile(path string) ([]domain.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseEvents(f)
}
