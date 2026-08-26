package store

import (
	"encoding/json"
	"time"

	"kilncurve-release/internal/domain"
)

const CurrentSchemaVersion = 1

type CommandResult struct {
	Command     string          `json:"command"`
	EntityID    string          `json:"entityId"`
	ProjectID   string          `json:"projectId"`
	Version     int             `json:"version"`
	Response    json.RawMessage `json:"response,omitempty"`
	CompletedAt time.Time       `json:"completedAt"`
}

type AuditEvent struct {
	Sequence       int       `json:"sequence"`
	At             time.Time `json:"at"`
	Actor          string    `json:"actor"`
	Action         string    `json:"action"`
	ProjectID      string    `json:"projectId"`
	EntityID       string    `json:"entityId"`
	PreviousDigest string    `json:"previousDigest"`
	Digest         string    `json:"digest"`
}

type State struct {
	SchemaVersion    int                                    `json:"schemaVersion"`
	Projects         map[string]*domain.TrialProject        `json:"projects"`
	Revisions        map[string]*domain.FiringCurveRevision `json:"revisions"`
	Runs             map[string]*domain.TrialRun            `json:"runs"`
	Deviations       map[string]*domain.Deviation           `json:"deviations"`
	DeviationBatches map[string]*domain.DeviationBatch      `json:"deviationBatches"`
	Cards            map[string]*domain.ProcessCard         `json:"cards"`
	Idempotency      map[string]CommandResult               `json:"idempotency"`
	Audits           []AuditEvent                           `json:"audits"`
}

func NewState() *State {
	return &State{SchemaVersion: CurrentSchemaVersion, Projects: map[string]*domain.TrialProject{}, Revisions: map[string]*domain.FiringCurveRevision{}, Runs: map[string]*domain.TrialRun{}, Deviations: map[string]*domain.Deviation{}, DeviationBatches: map[string]*domain.DeviationBatch{}, Cards: map[string]*domain.ProcessCard{}, Idempotency: map[string]CommandResult{}}
}

func (s *State) Clone() (*State, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var out State
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	out.ensureMaps()
	return &out, nil
}
func (s *State) ensureMaps() {
	if s.Projects == nil {
		s.Projects = map[string]*domain.TrialProject{}
	}
	if s.Revisions == nil {
		s.Revisions = map[string]*domain.FiringCurveRevision{}
	}
	if s.Runs == nil {
		s.Runs = map[string]*domain.TrialRun{}
	}
	if s.Deviations == nil {
		s.Deviations = map[string]*domain.Deviation{}
	}
	if s.DeviationBatches == nil {
		s.DeviationBatches = map[string]*domain.DeviationBatch{}
	}
	if s.Cards == nil {
		s.Cards = map[string]*domain.ProcessCard{}
	}
	if s.Idempotency == nil {
		s.Idempotency = map[string]CommandResult{}
	}
}
