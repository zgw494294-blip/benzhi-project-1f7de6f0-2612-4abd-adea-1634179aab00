package domain

import "time"

type Role string

const (
	RoleProcessEngineer Role = "PROCESS_ENGINEER"
	RoleTrialOperator   Role = "TRIAL_OPERATOR"
	RoleQualityReviewer Role = "QUALITY_REVIEWER"
)

func RequireRole(actual string, expected Role) error {
	if Role(actual) != expected {
		return NewError(ErrForbidden, "当前角色无权执行此操作，需要角色 "+string(expected), "role")
	}
	return nil
}

func (p *TrialProject) CurveFrozenApplied(now time.Time, actor, digest string, correction bool) {
	if correction {
		p.Status = ProjectCorrection
	} else {
		p.Status = ProjectCurveFrozen
	}
	p.Touch(now)
	p.AddEvent(now, "REVISION_FROZEN", actor, "冻结曲线修订版 "+shortDigest(digest))
}

func (p *TrialProject) EvaluationApplied(now time.Time, actor, summary string, reviewReady bool) {
	if reviewReady {
		p.Status = ProjectReview
	} else {
		p.Status = ProjectCorrection
	}
	p.Touch(now)
	p.AddEvent(now, "TRIAL_RUN_EVALUATED", actor, summary)
}

func (p *TrialProject) CorrectionApplied(now time.Time, actor, checkCode string) {
	p.Status = ProjectCorrection
	p.Touch(now)
	p.AddEvent(now, "DEVIATION_CORRECTED", actor, "登记偏差 "+checkCode+" 原因与纠正动作")
}

func (p *TrialProject) ReviewReturned(now time.Time, reviewer, comment string) {
	p.Status = ProjectCorrection
	p.Touch(now)
	p.AddEvent(now, "REVIEW_RETURNED", reviewer, "质量复核退回："+comment)
}

func (p *TrialProject) CardIssued(now time.Time, reviewer, cardID, cardNumber string) {
	p.ProcessCardID = cardID
	p.Status = ProjectApproved
	p.Touch(now)
	p.AddEvent(now, "PROCESS_CARD_ISSUED", reviewer, "批准并签发工艺卡 "+cardNumber)
}

func shortDigest(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func (p *TrialProject) CheckInvariants() error {
	if p.ID == "" || p.Code == "" || p.Owner == "" {
		return NewError(ErrInvalid, "课题标识、编号和负责人不能为空", "project")
	}
	if p.Version < 1 {
		return NewError(ErrInvalid, "课题版本必须大于零", "version")
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		return NewError(ErrInvalid, "课题更新时间不能早于创建时间", "updatedAt")
	}
	if p.Status == ProjectApproved && p.ProcessCardID == "" {
		return NewError(ErrInvalid, "已批准课题必须关联工艺卡", "processCardId")
	}
	return nil
}
