package analytics

import (
	"math"
	"sort"
	"time"

	"energyai/internal/domain"
)

type ValidationReport struct {
	Total         int `json:"total"`
	Valid         int `json:"valid"`
	MissingHours  int `json:"missing_hours"`
	Duplicates    int `json:"duplicates"`
	InvalidValues int `json:"invalid_values"`
}

func ValidateReadings(rs []domain.Reading) ([]domain.Reading, ValidationReport) {
	rep := ValidationReport{Total: len(rs)}
	sorted := append([]domain.Reading(nil), rs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Timestamp.Before(sorted[j].Timestamp) })

	clean := make([]domain.Reading, 0, len(sorted))
	for i, r := range sorted {
		if i > 0 && r.Timestamp.Equal(sorted[i-1].Timestamp) {
			rep.Duplicates++
			continue
		}
		if !validReading(r) {
			rep.InvalidValues++
			continue
		}
		clean = append(clean, r)
	}
	for i := 1; i < len(clean); i++ {
		gap := clean[i].Timestamp.Sub(clean[i-1].Timestamp)
		if gap > time.Hour {
			rep.MissingHours += int(gap/time.Hour) - 1
		}
	}
	rep.Valid = len(clean)
	return clean, rep
}

func validReading(r domain.Reading) bool {
	for _, v := range []float64{r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return r.ConsumptionKWh >= 0 && r.VoltageV > 0 && r.CurrentA >= 0 &&
		r.PowerFactor >= 0 && r.PowerFactor <= 1
}
