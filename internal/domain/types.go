package domain

import "time"

type ProjectStatus string

const (
	ProjectDraft       ProjectStatus = "DRAFT"
	ProjectCurveFrozen ProjectStatus = "CURVE_FROZEN"
	ProjectTesting     ProjectStatus = "TESTING"
	ProjectCorrection  ProjectStatus = "CORRECTION"
	ProjectReview      ProjectStatus = "REVIEW"
	ProjectApproved    ProjectStatus = "APPROVED"
)

type FreezeStatus string

const (
	CurveEditable FreezeStatus = "EDITABLE"
	CurveFrozen   FreezeStatus = "FROZEN"
)

type RunStatus string

const (
	RunRecording RunStatus = "RECORDING"
	RunEvaluated RunStatus = "EVALUATED"
)

type DeviationStatus string

const (
	DeviationOpen     DeviationStatus = "OPEN"
	DeviationActioned DeviationStatus = "ACTIONED"
	DeviationResolved DeviationStatus = "RESOLVED"
)

type KilnLimits struct {
	MinTemperature       float64 `json:"minTemperature"`
	MaxTemperature       float64 `json:"maxTemperature"`
	MaxHeatingRate       float64 `json:"maxHeatingRate"`
	MaxCoolingRate       float64 `json:"maxCoolingRate"`
	MaxHoldMinutes       int     `json:"maxHoldMinutes"`
	MaxCycleMinutes      int     `json:"maxCycleMinutes"`
	TemperatureTolerance float64 `json:"temperatureTolerance"`
}

type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type QualityLimits struct {
	WaterAbsorption     Range   `json:"waterAbsorption"`
	Shrinkage           Range   `json:"shrinkage"`
	MaxColorDifference  float64 `json:"maxColorDifference"`
	MaxDeformation      float64 `json:"maxDeformation"`
	AllowSurfaceDefects bool    `json:"allowSurfaceDefects"`
}

type CheckResult struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Observed string `json:"observed"`
	Boundary string `json:"boundary"`
	Location string `json:"location,omitempty"`
	Message  string `json:"message"`
}

type TimelineEvent struct {
	At      time.Time `json:"at"`
	Type    string    `json:"type"`
	Actor   string    `json:"actor"`
	Summary string    `json:"summary"`
}
