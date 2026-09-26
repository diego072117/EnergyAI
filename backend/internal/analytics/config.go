package analytics

import "time"

type Config struct {
	BaselineDays int

	ShiftThreshold   float64
	ShiftWindowHours int
	ShiftMinHours    int

	SpikeZ      float64
	SpikeMinDev float64

	NominalVoltage   float64
	VoltageTolerance float64 // fraction of nominal, e.g. 0.05 = ±5%
	CoMoveTolerance  float64 // max relative gap between current and kWh ratios
	PFDropThreshold  float64 // absolute power-factor drop considered relevant
	VoltageChangePct float64 // relative voltage change considered relevant (%)

	PowerRatioTolerance float64 // deviation of V·I·PF/kWh vs the meter's own ratio
	DQMinOccurrences    int     // occurrences for a check to count as a signal
	DQMinSignals        int     // signals needed to raise a DATA_QUALITY finding

	EventWindow       time.Duration
	AlignedEventHours float64
}

func DefaultConfig() Config {
	return Config{
		BaselineDays:        7,
		ShiftThreshold:      0.25,
		ShiftWindowHours:    6,
		ShiftMinHours:       6,
		SpikeZ:              4,
		SpikeMinDev:         0.30,
		NominalVoltage:      220,
		VoltageTolerance:    0.05,
		CoMoveTolerance:     0.20,
		PFDropThreshold:     0.10,
		VoltageChangePct:    1.0,
		PowerRatioTolerance: 0.35,
		DQMinOccurrences:    3,
		DQMinSignals:        2,
		EventWindow:         12 * time.Hour,
		AlignedEventHours:   3,
	}
}
