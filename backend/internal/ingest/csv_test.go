package ingest

import (
	"strings"
	"testing"
	"time"

	"energyai/internal/domain"
)

func TestParseReadings(t *testing.T) {
	in := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n" +
		"M-1,2026-09-01 00:00:00,23.5,221.9,101.28,0.954,OK\n" +
		"M-1,2026-09-01 01:00,20.11,221.15,100.49,0.935,\n"
	rs, err := ParseReadings(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("got %d readings", len(rs))
	}
	want := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	if !rs[1].Timestamp.Equal(want) || rs[1].Status != "OK" || rs[0].PowerFactor != 0.954 {
		t.Errorf("unexpected reading %+v", rs[1])
	}
}

func TestParseReadingsErrors(t *testing.T) {
	cases := map[string]string{
		"missing column": "meter_id,timestamp\nM-1,2026-09-01 00:00\n",
		"bad number":     "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor\nM-1,2026-09-01 00:00,abc,1,1,1\n",
		"bad time":       "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor\nM-1,yesterday,1,1,1,1\n",
		"empty meter":    "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor\n,2026-09-01 00:00,1,1,1,1\n",
	}
	for name, in := range cases {
		if _, err := ParseReadings(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseEvents(t *testing.T) {
	in := "\uFEFFmeter_id,event_timestamp,event_type,description\n" +
		"M-104,2026-09-11 00:00,OPERATIONAL_CHANGE,New production line activated\n" +
		"M-109,2026-09-12 14:00,,No operational event reported\n"
	evs, err := ParseEvents(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 || evs[0].Type != domain.EventOperationalChange || evs[1].Type != domain.EventUnknown {
		t.Fatalf("unexpected events %+v", evs)
	}
}

func TestReadFilesFromRepoData(t *testing.T) {
	rs, err := ReadReadingsFile("../../data/readings.csv")
	if err != nil || len(rs) != 4032 {
		t.Fatalf("readings: %d, %v", len(rs), err)
	}
	evs, err := ReadEventsFile("../../data/events.csv")
	if err != nil || len(evs) != 4 {
		t.Fatalf("events: %d, %v", len(evs), err)
	}
	if _, err := ReadReadingsFile("missing.csv"); err == nil {
		t.Error("expected error for missing file")
	}
}
