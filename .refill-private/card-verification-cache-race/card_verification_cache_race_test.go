package cardverificationcacherace

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestCardVerificationCacheConcurrentColdMisses(t *testing.T) {
	const cardCount = 64
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	cardIDs := seedApprovedCards(t, repo, cardCount)
	service := application.NewService(repo)

	start := make(chan struct{})
	errs := make(chan error, cardCount)
	var workers sync.WaitGroup
	workers.Add(len(cardIDs))
	for _, cardID := range cardIDs {
		go func(id string) {
			defer workers.Done()
			<-start
			_, valid, verifyErr := service.VerifyCard(id)
			if verifyErr != nil {
				errs <- verifyErr
				return
			}
			if !valid {
				errs <- fmt.Errorf("工艺卡 %s 核验结果无效", id)
			}
		}(cardID)
	}
	close(start)
	workers.Wait()
	close(errs)
	for workerErr := range errs {
		t.Error(workerErr)
	}
}

func seedApprovedCards(t *testing.T, repo *store.Repository, count int) []string {
	t.Helper()
	at := time.Date(2026, 8, 26, 9, 30, 0, 0, time.UTC)
	ids := make([]string, 0, count)
	err := repo.Update(store.CommitMeta{}, func(state *store.State) error {
		for index := 0; index < count; index++ {
			projectID := fmt.Sprintf("project-%03d", index)
			cardID := fmt.Sprintf("card-%03d", index)
			project, createErr := domain.CreateProject(projectID, domain.NewProject{
				Code:          fmt.Sprintf("KC-RACE-%03d", index),
				Title:         "并发核验夹具",
				Owner:         "工艺工程师",
				BodyMaterial:  "瓷坯",
				GlazeMaterial: "透明釉",
				LoadingMethod: "平码",
				KilnLimits:    domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 90, MaxCycleMinutes: 600, TemperatureTolerance: 15},
				QualityLimits: domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 0.5}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1},
			}, at)
			if createErr != nil {
				return createErr
			}
			project.Status = domain.ProjectReview
			curve := domain.NewRevision(fmt.Sprintf("curve-%03d", index), projectID, 1, "", "工艺工程师", []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1200, DurationMinutes: 180}}, at)
			card, issueErr := domain.IssueCard(cardID, fmt.Sprintf("CARD-%03d", index), project, curve, []string{"trial-run:fixture"}, "质量复核员", at)
			if issueErr != nil {
				return issueErr
			}
			project.CardIssued(at, "质量复核员", card.ID, card.CardNumber)
			state.Projects[project.ID] = project
			state.Cards[card.ID] = card
			ids = append(ids, card.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids
}
