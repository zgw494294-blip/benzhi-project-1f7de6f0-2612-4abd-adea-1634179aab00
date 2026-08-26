package verification

import (
	"sort"

	"kilncurve-release/internal/domain"
)

type CheckMatrixRow struct {
	Code            string                 `json:"code"`
	Name            string                 `json:"name"`
	LatestRunID     string                 `json:"latestRunId"`
	Passed          bool                   `json:"passed"`
	Observed        string                 `json:"observed"`
	Boundary        string                 `json:"boundary"`
	DeviationID     string                 `json:"deviationId,omitempty"`
	DeviationStatus domain.DeviationStatus `json:"deviationStatus,omitempty"`
}

type EvidenceSummary struct {
	Complete        bool             `json:"complete"`
	ReviewReady     bool             `json:"reviewReady"`
	TotalChecks     int              `json:"totalChecks"`
	PassedChecks    int              `json:"passedChecks"`
	OpenDeviations  int              `json:"openDeviations"`
	Rows            []CheckMatrixRow `json:"rows"`
	BlockingReasons []string         `json:"blockingReasons"`
}

func SummarizeEvidence(project *domain.TrialProject, runs map[string]*domain.TrialRun, deviations map[string]*domain.Deviation) EvidenceSummary {
	summary := EvidenceSummary{}
	latest := map[string]CheckMatrixRow{}
	for _, runID := range project.RunIDs {
		run := runs[runID]
		if run == nil {
			summary.BlockingReasons = append(summary.BlockingReasons, "课题引用的试烧轮次不存在")
			continue
		}
		if run.Status != domain.RunEvaluated {
			summary.BlockingReasons = append(summary.BlockingReasons, "存在未完成评估的试烧轮次")
		}
		for _, check := range run.Checks {
			latest[check.Code] = CheckMatrixRow{Code: check.Code, Name: check.Name, LatestRunID: run.ID, Passed: check.Passed, Observed: check.Observed, Boundary: check.Boundary}
		}
	}
	for _, deviationID := range project.DeviationIDs {
		deviation := deviations[deviationID]
		if deviation == nil {
			summary.BlockingReasons = append(summary.BlockingReasons, "课题引用的偏差不存在")
			continue
		}
		row := latest[deviation.CheckCode]
		row.DeviationID = deviation.ID
		row.DeviationStatus = deviation.Status
		latest[deviation.CheckCode] = row
		if deviation.Status != domain.DeviationResolved {
			summary.OpenDeviations++
		}
	}
	for _, code := range allChecks {
		if row, ok := latest[code]; ok {
			summary.Rows = append(summary.Rows, row)
		}
	}
	sort.SliceStable(summary.Rows, func(i, j int) bool { return index(summary.Rows[i].Code) < index(summary.Rows[j].Code) })
	summary.TotalChecks = len(summary.Rows)
	for _, row := range summary.Rows {
		if row.Passed && (row.DeviationID == "" || row.DeviationStatus == domain.DeviationResolved) {
			summary.PassedChecks++
		}
	}
	if len(project.RunIDs) == 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "尚未登记试烧证据")
	}
	if summary.TotalChecks < len(allChecks) {
		summary.BlockingReasons = append(summary.BlockingReasons, "首轮全项检查证据不完整")
	}
	if summary.OpenDeviations > 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "仍有未关闭偏差")
	}
	summary.Complete = summary.TotalChecks == len(allChecks)
	summary.ReviewReady = summary.Complete && summary.OpenDeviations == 0 && len(summary.BlockingReasons) == 0
	return summary
}
