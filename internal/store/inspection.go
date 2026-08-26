package store

type Statistics struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Projects           int    `json:"projects"`
	Revisions          int    `json:"revisions"`
	Runs               int    `json:"runs"`
	Deviations         int    `json:"deviations"`
	DeviationBatches   int    `json:"deviationBatches"`
	Cards              int    `json:"cards"`
	IdempotencyResults int    `json:"idempotencyResults"`
	AuditEvents        int    `json:"auditEvents"`
	AuditHead          string `json:"auditHead,omitempty"`
}

func (r *Repository) Inspect() (Statistics, error) {
	var result Statistics
	err := r.Read(func(state *State) error {
		if err := ValidateState(state); err != nil {
			return err
		}
		result = Statistics{SchemaVersion: state.SchemaVersion, Projects: len(state.Projects), Revisions: len(state.Revisions), Runs: len(state.Runs), Deviations: len(state.Deviations), DeviationBatches: len(state.DeviationBatches), Cards: len(state.Cards), IdempotencyResults: len(state.Idempotency), AuditEvents: len(state.Audits)}
		if len(state.Audits) > 0 {
			result.AuditHead = state.Audits[len(state.Audits)-1].Digest
		}
		return nil
	})
	return result, err
}

func (r *Repository) ProjectAudits(projectID string) ([]AuditEvent, error) {
	result := []AuditEvent{}
	err := r.Read(func(state *State) error {
		if state.Projects[projectID] == nil {
			return nil
		}
		for _, event := range state.Audits {
			if event.ProjectID == projectID || event.EntityID == projectID {
				result = append(result, event)
			}
		}
		return nil
	})
	return result, err
}
