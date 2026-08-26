package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type ApplicabilitySnapshot struct {
	ProjectCode   string        `json:"projectCode"`
	BodyMaterial  string        `json:"bodyMaterial"`
	GlazeMaterial string        `json:"glazeMaterial"`
	LoadingMethod string        `json:"loadingMethod"`
	KilnLimits    KilnLimits    `json:"kilnLimits"`
	QualityLimits QualityLimits `json:"qualityLimits"`
}

type ProcessCard struct {
	ID                    string                `json:"id"`
	ProjectID             string                `json:"projectId"`
	CardNumber            string                `json:"cardNumber"`
	CurveSnapshot         json.RawMessage       `json:"curveSnapshot"`
	ApplicabilitySnapshot ApplicabilitySnapshot `json:"applicabilitySnapshot"`
	EvidenceRefs          []string              `json:"evidenceRefs"`
	Reviewer              string                `json:"reviewer"`
	IssuedAt              time.Time             `json:"issuedAt"`
	Digest                string                `json:"digest"`
	VerificationVersion   string                `json:"verificationVersion"`
}

func IssueCard(id, number string, project *TrialProject, curve *FiringCurveRevision, evidence []string, reviewer string, at time.Time) (*ProcessCard, error) {
	if project.Status != ProjectReview {
		return nil, NewError(ErrState, "课题尚未进入质量复核", "status")
	}
	if reviewer == "" {
		return nil, NewError(ErrInvalid, "复核员不能为空", "reviewer")
	}
	curveBytes, err := curve.CanonicalSnapshot()
	if err != nil {
		return nil, err
	}
	c := &ProcessCard{ID: id, ProjectID: project.ID, CardNumber: number, CurveSnapshot: curveBytes, ApplicabilitySnapshot: ApplicabilitySnapshot{ProjectCode: project.Code, BodyMaterial: project.BodyMaterial, GlazeMaterial: project.GlazeMaterial, LoadingMethod: project.LoadingMethod, KilnLimits: project.KilnLimits, QualityLimits: project.QualityLimits}, EvidenceRefs: append([]string(nil), evidence...), Reviewer: reviewer, IssuedAt: at, VerificationVersion: "kilncurve-verification/v1"}
	digest, err := c.CalculateDigest()
	if err != nil {
		return nil, err
	}
	c.Digest = digest
	return c, nil
}

func (c *ProcessCard) CalculateDigest() (string, error) {
	type input struct {
		ID            string                `json:"id"`
		ProjectID     string                `json:"projectId"`
		CardNumber    string                `json:"cardNumber"`
		Curve         json.RawMessage       `json:"curve"`
		Applicability ApplicabilitySnapshot `json:"applicability"`
		Evidence      []string              `json:"evidence"`
		Reviewer      string                `json:"reviewer"`
		IssuedAt      time.Time             `json:"issuedAt"`
		Version       string                `json:"version"`
	}
	b, err := json.Marshal(input{c.ID, c.ProjectID, c.CardNumber, c.CurveSnapshot, c.ApplicabilitySnapshot, c.EvidenceRefs, c.Reviewer, c.IssuedAt, c.VerificationVersion})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func (c *ProcessCard) Verify() (bool, error) {
	d, err := c.CalculateDigest()
	return err == nil && d == c.Digest, err
}
