package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"kilncurve-release/internal/domain"
)

type selfProject struct {
	ID      string               `json:"id"`
	Version int                  `json:"version"`
	Status  domain.ProjectStatus `json:"status"`
}
type selfRevision struct {
	ID            string `json:"id"`
	ContentDigest string `json:"contentDigest"`
}
type selfDetail struct {
	Project       selfProject         `json:"project"`
	Deviations    []domain.Deviation  `json:"deviations"`
	ProcessCard   *domain.ProcessCard `json:"processCard"`
	CardIntegrity *bool               `json:"cardIntegrity"`
}

func selfcheckFlow(ctx context.Context, client *http.Client, base string) error {
	var health map[string]any
	if err := selfRequest(ctx, client, http.MethodGet, base+"/api/health", nil, &health); err != nil {
		return err
	}
	projectInput := map[string]any{"idempotencyKey": "self-project", "role": "PROCESS_ENGINEER", "code": "SELF-001", "title": "自检日用瓷曲线", "owner": "工艺工程师", "bodyMaterial": "高岭土坯体", "glazeMaterial": "透明釉", "loadingMethod": "棚板平码", "kilnLimits": domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 180, MaxCycleMinutes: 600, TemperatureTolerance: 15}, "qualityLimits": domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 0.5}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1, AllowSurfaceDefects: false}}
	var p selfProject
	if err := selfRequest(ctx, client, http.MethodPost, base+"/api/projects", projectInput, &p); err != nil {
		return err
	}
	segments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1200, DurationMinutes: 120}, {Order: 2, Kind: domain.SegmentHold, StartTemperature: 1200, EndTemperature: 1200, DurationMinutes: 30}, {Order: 3, Kind: domain.SegmentCool, StartTemperature: 1200, EndTemperature: 20, DurationMinutes: 180}}
	var rev selfRevision
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/revisions", base, p.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-revision-1", "role": "PROCESS_ENGINEER", "actor": "工艺工程师", "segments": segments}, &rev); err != nil {
		return err
	}
	p.Version++
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/revisions/%s/freeze", base, p.ID, rev.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-freeze-1", "role": "PROCESS_ENGINEER", "actor": "工艺工程师"}, &rev); err != nil {
		return err
	}
	if len(rev.ContentDigest) != 64 {
		return fmt.Errorf("冻结摘要长度异常")
	}
	p.Version++
	samples := []domain.TemperatureSample{{Minute: 0, Temperature: 20}, {Minute: 120, Temperature: 1200}, {Minute: 150, Temperature: 1200}, {Minute: 330, Temperature: 20}}
	var run map[string]any
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/trial-runs", base, p.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-run-1", "role": "TRIAL_OPERATOR", "curveRevisionId": rev.ID, "temperatureSamples": samples, "qualityMeasurements": qualityValues(0.2, 12, 3.0, 0.4, false), "operator": "试烧操作员"}, &run); err != nil {
		return err
	}
	p.Version++
	var d selfDetail
	if err := selfRequest(ctx, client, http.MethodGet, fmt.Sprintf("%s/api/projects/%s", base, p.ID), nil, &d); err != nil {
		return err
	}
	if len(d.Deviations) != 1 || d.Deviations[0].CheckCode != "COLOR_DIFFERENCE" {
		return fmt.Errorf("预期产生一个色差偏差")
	}
	var rev2 selfRevision
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/revisions", base, p.ID), map[string]any{"expectedVersion": d.Project.Version, "idempotencyKey": "self-revision-2", "role": "PROCESS_ENGINEER", "actor": "工艺工程师", "basedOnRevisionId": rev.ID, "segments": segments}, &rev2); err != nil {
		return err
	}
	p.Version = d.Project.Version + 1
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/revisions/%s/freeze", base, p.ID, rev2.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-freeze-2", "role": "PROCESS_ENGINEER", "actor": "工艺工程师"}, &rev2); err != nil {
		return err
	}
	p.Version++
	dev := d.Deviations[0]
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/deviations/%s/correct", base, p.ID, dev.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-correct", "role": "PROCESS_ENGINEER", "actor": "工艺工程师", "cause": "釉料配比波动", "correctiveAction": "校正釉浆比重并复测", "relatedRevisionId": rev2.ID}, &dev); err != nil {
		return err
	}
	p.Version++
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/trial-runs", base, p.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-retest", "role": "TRIAL_OPERATOR", "curveRevisionId": rev2.ID, "scopeCheckCodes": []string{"COLOR_DIFFERENCE"}, "temperatureSamples": samples, "qualityMeasurements": qualityValues(0.2, 12, 0.8, 0.4, false), "operator": "试烧操作员"}, &run); err != nil {
		return err
	}
	p.Version++
	var review map[string]json.RawMessage
	if err := selfRequest(ctx, client, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/review", base, p.ID), map[string]any{"expectedVersion": p.Version, "idempotencyKey": "self-review", "role": "QUALITY_REVIEWER", "reviewer": "质量复核员", "decision": "APPROVE", "comment": "证据完整，同意定版"}, &review); err != nil {
		return err
	}
	if err := selfRequest(ctx, client, http.MethodGet, fmt.Sprintf("%s/api/projects/%s", base, p.ID), nil, &d); err != nil {
		return err
	}
	if d.Project.Status != domain.ProjectApproved || d.ProcessCard == nil || d.CardIntegrity == nil || !*d.CardIntegrity {
		return fmt.Errorf("工艺卡未签发或完整性核验失败")
	}
	var verified struct {
		Valid bool `json:"valid"`
	}
	if err := selfRequest(ctx, client, http.MethodGet, fmt.Sprintf("%s/api/process-cards/%s/verify", base, d.ProcessCard.ID), nil, &verified); err != nil {
		return err
	}
	if !verified.Valid {
		return fmt.Errorf("独立工艺卡核验失败")
	}
	return nil
}

func qualityValues(water, shrink, color, deformation float64, defects bool) domain.QualityMeasurements {
	return domain.QualityMeasurements{WaterAbsorption: &water, Shrinkage: &shrink, ColorDifference: &color, Deformation: &deformation, SurfaceDefects: &defects}
}
func selfRequest(ctx context.Context, client *http.Client, method, url string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", method, url, resp.StatusCode, string(b))
	}
	if output != nil && len(b) > 0 {
		if err = json.Unmarshal(b, output); err != nil {
			return fmt.Errorf("解析响应: %w", err)
		}
	}
	return nil
}
