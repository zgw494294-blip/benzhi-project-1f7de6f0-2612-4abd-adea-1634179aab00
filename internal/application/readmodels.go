package application

import (
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/verification"
)

type AvailableActions struct {
	EditBoundaries   bool `json:"editBoundaries"`
	EditCurve        bool `json:"editCurve"`
	FreezeCurve      bool `json:"freezeCurve"`
	RecordTrial      bool `json:"recordTrial"`
	CorrectDeviation bool `json:"correctDeviation"`
	CreateRetest     bool `json:"createRetest"`
	Review           bool `json:"review"`
	VerifyCard       bool `json:"verifyCard"`
}

type ProjectOverview struct {
	StatusLabel            string           `json:"statusLabel"`
	RevisionCount          int              `json:"revisionCount"`
	FrozenRevisionCount    int              `json:"frozenRevisionCount"`
	TrialRunCount          int              `json:"trialRunCount"`
	OpenDeviationCount     int              `json:"openDeviationCount"`
	ResolvedDeviationCount int              `json:"resolvedDeviationCount"`
	LatestFrozenRevisionID string           `json:"latestFrozenRevisionId,omitempty"`
	Actions                AvailableActions `json:"actions"`
}

type RevisionView struct {
	ID                string              `json:"id"`
	RevisionNo        int                 `json:"revisionNo"`
	FreezeStatus      domain.FreezeStatus `json:"freezeStatus"`
	BasedOnRevisionID string              `json:"basedOnRevisionId,omitempty"`
	Digest            string              `json:"digest,omitempty"`
	Metrics           domain.CurveMetrics `json:"metrics"`
	BoundaryPoints    []domain.CurvePoint `json:"boundaryPoints"`
}

type RunView struct {
	ID                      string                  `json:"id"`
	RunNo                   int                     `json:"runNo"`
	CurveRevisionID         string                  `json:"curveRevisionId"`
	Retest                  bool                    `json:"retest"`
	Scope                   []string                `json:"scope,omitempty"`
	PassedChecks            int                     `json:"passedChecks"`
	FailedChecks            int                     `json:"failedChecks"`
	PeakMeasuredTemperature float64                 `json:"peakMeasuredTemperature"`
	Status                  domain.RunStatus        `json:"status"`
	EvidenceProgress        domain.EvidenceProgress `json:"evidenceProgress"`
	DeviationBatchID        string                  `json:"deviationBatchId,omitempty"`
}

func buildOverview(project *domain.TrialProject, revisions []*domain.FiringCurveRevision, deviations []*domain.Deviation) ProjectOverview {
	view := ProjectOverview{StatusLabel: statusLabel(project.Status), RevisionCount: len(revisions), TrialRunCount: len(project.RunIDs)}
	for _, revision := range revisions {
		if revision.FreezeStatus == domain.CurveFrozen {
			view.FrozenRevisionCount++
			view.LatestFrozenRevisionID = revision.ID
		}
	}
	for _, deviation := range deviations {
		if deviation.Status == domain.DeviationResolved {
			view.ResolvedDeviationCount++
		} else {
			view.OpenDeviationCount++
		}
	}
	view.Actions = AvailableActions{EditBoundaries: project.Status == domain.ProjectDraft && view.FrozenRevisionCount == 0, EditCurve: project.Status == domain.ProjectDraft || project.Status == domain.ProjectCorrection, FreezeCurve: project.Status == domain.ProjectDraft || project.Status == domain.ProjectCorrection, RecordTrial: project.Status == domain.ProjectCurveFrozen || project.Status == domain.ProjectTesting, CorrectDeviation: project.Status == domain.ProjectCorrection && view.OpenDeviationCount > 0, CreateRetest: project.Status == domain.ProjectCorrection, Review: project.Status == domain.ProjectReview, VerifyCard: project.Status == domain.ProjectApproved}
	return view
}

func buildRevisionViews(revisions []*domain.FiringCurveRevision) []RevisionView {
	views := make([]RevisionView, 0, len(revisions))
	for _, revision := range revisions {
		views = append(views, RevisionView{ID: revision.ID, RevisionNo: revision.RevisionNo, FreezeStatus: revision.FreezeStatus, BasedOnRevisionID: revision.BasedOnRevisionID, Digest: revision.ContentDigest, Metrics: domain.CalculateCurveMetrics(revision.Segments), BoundaryPoints: domain.CurveBoundaryPoints(revision.Segments)})
	}
	return views
}

func buildRunViews(runs []*domain.TrialRun) []RunView {
	views := make([]RunView, 0, len(runs))
	for _, run := range runs {
		view := RunView{ID: run.ID, RunNo: run.RunNo, CurveRevisionID: run.CurveRevisionID, Retest: len(run.ScopeCheckCodes) > 0, Scope: append([]string(nil), run.ScopeCheckCodes...), Status: run.Status, EvidenceProgress: run.EvidenceProgress(), DeviationBatchID: run.DeviationBatchID}
		for _, check := range run.Checks {
			if check.Passed {
				view.PassedChecks++
			} else {
				view.FailedChecks++
			}
		}
		for _, sample := range run.TemperatureSamples {
			if sample.Temperature > view.PeakMeasuredTemperature {
				view.PeakMeasuredTemperature = sample.Temperature
			}
		}
		views = append(views, view)
	}
	return views
}

func statusLabel(status domain.ProjectStatus) string {
	labels := map[domain.ProjectStatus]string{domain.ProjectDraft: "草拟", domain.ProjectCurveFrozen: "曲线已冻结", domain.ProjectTesting: "试烧中", domain.ProjectCorrection: "偏差整改", domain.ProjectReview: "待质量复核", domain.ProjectApproved: "已批准"}
	if label, ok := labels[status]; ok {
		return label
	}
	return string(status)
}

func evidenceCanReview(summary verification.EvidenceSummary) bool {
	return summary.ReviewReady && summary.Complete && summary.OpenDeviations == 0
}
