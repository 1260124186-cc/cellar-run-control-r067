package domain

import "time"

const (
	GravityReferenceTemperatureC = 20.0
	GravityCorrectionCoefficient = 0.00035
	GravityCorrectionVersion     = 1
)

type GravityCorrection struct {
	ReferenceTemperatureC float64 `json:"reference_temperature_c"`
	Coefficient           float64 `json:"coefficient"`
	Version               int64   `json:"version"`
}

func DefaultGravityCorrection() GravityCorrection {
	return GravityCorrection{
		ReferenceTemperatureC: GravityReferenceTemperatureC,
		Coefficient:           GravityCorrectionCoefficient,
		Version:               GravityCorrectionVersion,
	}
}

func (c GravityCorrection) Valid() bool {
	return c.ReferenceTemperatureC > 0 &&
		c.Coefficient > 0 &&
		c.Version > 0
}

func StandardGravityAtCorrection(
	observedGravity float64,
	observedTemperatureC float64,
	correction GravityCorrection,
) float64 {
	return observedGravity +
		correction.Coefficient*(observedTemperatureC-correction.ReferenceTemperatureC)
}

type GravityObservation struct {
	RunID           string            `json:"run_id"`
	Sequence        int               `json:"sequence"`
	Gravity         float64           `json:"gravity"`
	TemperatureC    float64           `json:"temperature_c"`
	StandardGravity float64           `json:"standard_gravity"`
	Correction      GravityCorrection `json:"gravity_correction"`
	ObservedAt      time.Time         `json:"observed_at"`
	Note            string            `json:"note,omitempty"`
	ReceivedAt      time.Time         `json:"received_at"`
}

func (o GravityObservation) HasConfirmedStandardGravity() bool {
	return o.StandardGravity != 0 && o.Correction.Valid()
}

// CorrectedGravity returns the temperature-corrected gravity captured when the
// observation was accepted. It is the single value shared by run detail, run
// list, and completion metrics, and it never changes after acceptance.
func (o GravityObservation) CorrectedGravity() float64 {
	return o.StandardGravity
}

func (o GravityObservation) WithStandardGravity(
	correction GravityCorrection,
) GravityObservation {
	o.StandardGravity = StandardGravityAtCorrection(
		o.Gravity,
		o.TemperatureC,
		correction,
	)
	o.Correction = correction
	return o
}

func ValidateObservation(
	runStartedAt time.Time,
	previous *GravityObservation,
	candidate GravityObservation,
	now time.Time,
) error {
	if candidate.Gravity < 0.980 || candidate.Gravity > 1.200 {
		return FieldError(CodeInvalidInput, "gravity",
			"must be between 0.980 and 1.200")
	}
	if candidate.TemperatureC < -5 || candidate.TemperatureC > 60 {
		return FieldError(CodeInvalidInput, "temperature_c",
			"must be between -5 and 60 Celsius")
	}
	if len([]rune(candidate.Note)) > 500 {
		return FieldError(CodeInvalidInput, "note",
			"must not exceed 500 characters")
	}
	observed := candidate.ObservedAt.UTC()
	if observed.Before(runStartedAt.UTC()) {
		return FieldError(CodeInvalidInput, "observed_at",
			"must not precede the run start")
	}
	if observed.After(now.UTC().Add(time.Minute)) {
		return FieldError(CodeInvalidInput, "observed_at",
			"must not be in the future")
	}
	if previous != nil {
		if !observed.After(previous.ObservedAt.UTC()) {
			return NewError(CodeObservationConflict,
				"observation time must increase strictly")
		}
		if candidate.Gravity-previous.Gravity > 0.010 {
			return NewError(CodeObservationConflict,
				"gravity must not rise more than 0.010 above the previous value")
		}
	}
	return nil
}
