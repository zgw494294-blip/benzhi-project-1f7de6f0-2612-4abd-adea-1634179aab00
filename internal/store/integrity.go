package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func auditDigest(e AuditEvent) (string, error) {
	type input struct {
		Sequence       int    `json:"sequence"`
		At             string `json:"at"`
		Actor          string `json:"actor"`
		Action         string `json:"action"`
		ProjectID      string `json:"projectId"`
		EntityID       string `json:"entityId"`
		PreviousDigest string `json:"previousDigest"`
	}
	b, err := json.Marshal(input{e.Sequence, e.At.UTC().Format("2006-01-02T15:04:05.000000000Z"), e.Actor, e.Action, e.ProjectID, e.EntityID, e.PreviousDigest})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func appendAudit(s *State, e AuditEvent) error {
	e.Sequence = len(s.Audits) + 1
	if len(s.Audits) > 0 {
		e.PreviousDigest = s.Audits[len(s.Audits)-1].Digest
	}
	d, err := auditDigest(e)
	if err != nil {
		return err
	}
	e.Digest = d
	s.Audits = append(s.Audits, e)
	return nil
}

func ValidateState(s *State) error {
	if s.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("不支持的 schemaVersion %d，当前版本 %d", s.SchemaVersion, CurrentSchemaVersion)
	}
	s.ensureMaps()
	for id, p := range s.Projects {
		if err := p.CheckInvariants(); err != nil {
			return fmt.Errorf("课题 %s 不变量失败: %w", id, err)
		}
		if id != p.ID {
			return fmt.Errorf("课题键与 ID 不一致: %s", id)
		}
		for _, rid := range p.RevisionIDs {
			r := s.Revisions[rid]
			if r == nil || r.ProjectID != p.ID {
				return fmt.Errorf("课题 %s 引用无效曲线 %s", id, rid)
			}
			if err := r.VerifyFrozenIntegrity(); err != nil {
				return fmt.Errorf("曲线 %s 冻结完整性校验失败: %w", rid, err)
			}
		}
		for _, runID := range p.RunIDs {
			r := s.Runs[runID]
			if r == nil || r.ProjectID != p.ID {
				return fmt.Errorf("课题 %s 引用无效试烧 %s", id, runID)
			}
		}
		for _, did := range p.DeviationIDs {
			if s.Deviations[did] == nil {
				return fmt.Errorf("课题 %s 引用无效偏差 %s", id, did)
			}
		}
		for _, batchID := range p.DeviationBatchIDs {
			batch := s.DeviationBatches[batchID]
			if batch == nil || batch.ProjectID != p.ID {
				return fmt.Errorf("课题 %s 引用无效整改批次 %s", id, batchID)
			}
		}
		if p.ProcessCardID != "" {
			c := s.Cards[p.ProcessCardID]
			if c == nil || c.ProjectID != p.ID {
				return fmt.Errorf("课题 %s 引用无效工艺卡", id)
			}
		}
	}
	for id, r := range s.Runs {
		revision := s.Revisions[r.CurveRevisionID]
		if s.Projects[r.ProjectID] == nil || revision == nil || revision.ProjectID != r.ProjectID {
			return fmt.Errorf("试烧 %s 实体引用损坏", id)
		}
		if r.DeviationBatchID != "" {
			batch := s.DeviationBatches[r.DeviationBatchID]
			if batch == nil || batch.ProjectID != r.ProjectID {
				return fmt.Errorf("试烧 %s 引用无效整改批次", id)
			}
		}
	}
	for id, d := range s.Deviations {
		if s.Runs[d.TrialRunID] == nil {
			return fmt.Errorf("偏差 %s 引用无效试烧", id)
		}
		if d.RetestRunID != "" && s.Runs[d.RetestRunID] == nil {
			return fmt.Errorf("偏差 %s 引用无效复试", id)
		}
		if d.BatchID != "" && s.DeviationBatches[d.BatchID] == nil {
			return fmt.Errorf("偏差 %s 引用无效整改批次", id)
		}
	}
	for id, batch := range s.DeviationBatches {
		project := s.Projects[batch.ProjectID]
		revision := s.Revisions[batch.RelatedRevisionID]
		if project == nil || revision == nil || revision.ProjectID != batch.ProjectID {
			return fmt.Errorf("整改批次 %s 实体引用损坏", id)
		}
		for _, deviationID := range batch.DeviationIDs {
			deviation := s.Deviations[deviationID]
			if deviation == nil || !stringSliceContains(project.DeviationIDs, deviationID) {
				return fmt.Errorf("整改批次 %s 引用无效偏差 %s", id, deviationID)
			}
		}
	}
	prev := ""
	for i, e := range s.Audits {
		if e.Sequence != i+1 || e.PreviousDigest != prev {
			return fmt.Errorf("审计链序号或前序摘要损坏: %d", i+1)
		}
		d, err := auditDigest(e)
		if err != nil || d != e.Digest {
			return fmt.Errorf("审计链摘要损坏: %d", i+1)
		}
		prev = e.Digest
	}
	for id, c := range s.Cards {
		ok, err := c.Verify()
		if err != nil || !ok {
			return fmt.Errorf("工艺卡 %s 完整性校验失败", id)
		}
	}
	return nil
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
