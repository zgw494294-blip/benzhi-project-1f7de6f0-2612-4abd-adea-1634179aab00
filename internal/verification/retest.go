package verification

import (
	"sort"

	"kilncurve-release/internal/domain"
)

func ValidateRetestScope(scope []string, deviations []*domain.Deviation) error {
	if len(scope) == 0 {
		return domain.NewError(domain.ErrInvalid, "定向复试范围不能为空", "scopeCheckCodes")
	}
	open := map[string]bool{}
	for _, d := range deviations {
		if d.Status == domain.DeviationActioned {
			open[d.CheckCode] = true
		}
	}
	seen := map[string]bool{}
	for _, code := range scope {
		if seen[code] {
			return domain.NewError(domain.ErrInvalid, "定向复试范围包含重复检查项", "scopeCheckCodes")
		}
		seen[code] = true
		if !open[code] {
			return domain.NewError(domain.ErrInvalid, "定向复试只能覆盖已处置且未关闭的失败检查项", "scopeCheckCodes")
		}
	}
	return nil
}

func BuildBatchRetestScope(batch *domain.DeviationBatch, deviations []*domain.Deviation) ([]string, error) {
	if batch == nil {
		return nil, domain.NewError(domain.ErrInvalid, "整改批次不存在", "deviationBatchId")
	}
	members := make(map[string]bool, len(batch.DeviationIDs))
	for _, id := range batch.DeviationIDs {
		members[id] = true
	}
	wanted := map[string]bool{}
	for _, deviation := range deviations {
		if !members[deviation.ID] {
			continue
		}
		if deviation.BatchID != batch.ID {
			return nil, domain.NewError(domain.ErrInvalid, "偏差不属于指定整改批次", "deviationBatchId")
		}
		if deviation.Status == domain.DeviationActioned {
			wanted[deviation.CheckCode] = true
		}
	}
	out := make([]string, 0, len(wanted))
	for _, code := range AllCheckCodes() {
		if wanted[code] {
			out = append(out, code)
		}
	}
	if len(out) == 0 {
		return nil, domain.NewError(domain.ErrState, "整改批次没有可复试的已处置检查项", "status")
	}
	return out, nil
}

func ResolveRetest(run *domain.TrialRun, deviations []*domain.Deviation, at interface{ IsZero() bool }) []string {
	_ = at
	failed := map[string]bool{}
	for _, c := range run.Checks {
		if !c.Passed {
			failed[c.Code] = true
		}
	}
	resolved := []string{}
	for _, d := range deviations {
		if d.RetestRunID == run.ID && !failed[d.CheckCode] {
			resolved = append(resolved, d.CheckCode)
		}
	}
	sort.Strings(resolved)
	return resolved
}

func ReviewReady(project *domain.TrialProject, runs map[string]*domain.TrialRun, deviations map[string]*domain.Deviation) error {
	if len(project.RunIDs) == 0 {
		return domain.NewError(domain.ErrState, "尚无试烧证据", "runs")
	}
	for _, id := range project.RunIDs {
		r := runs[id]
		if r == nil || r.Status != domain.RunEvaluated {
			return domain.NewError(domain.ErrState, "存在未完成评估的试烧轮次", "runs")
		}
	}
	for _, id := range project.DeviationIDs {
		d := deviations[id]
		if d == nil || d.Status != domain.DeviationResolved {
			return domain.NewError(domain.ErrState, "仍有未关闭偏差，不能复核", "deviations")
		}
	}
	return nil
}
