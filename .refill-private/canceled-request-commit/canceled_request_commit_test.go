package canceled_request_commit_test

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/web"
)

type observedBody struct {
	reader  *bytes.Reader
	started chan struct{}
	once    sync.Once
}

func (b *observedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.started) })
	return b.reader.Read(p)
}

func TestCanceledRequestCannotCommitProject(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	handler := web.NewHandler(service)

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- repo.Update(store.CommitMeta{}, func(*store.State) error {
			close(lockHeld)
			<-releaseLock
			return nil
		})
	}()
	<-lockHeld

	payload := []byte(`{"idempotencyKey":"cancel-create","role":"PROCESS_ENGINEER","code":"CANCEL-1","title":"取消请求","owner":"工程师","bodyMaterial":"坯","glazeMaterial":"釉","loadingMethod":"平码","kilnLimits":{"minTemperature":20,"maxTemperature":1300,"maxHeatingRate":10,"maxCoolingRate":8,"maxHoldMinutes":120,"maxCycleMinutes":500,"temperatureTolerance":10},"qualityLimits":{"waterAbsorption":{"min":0,"max":1},"shrinkage":{"min":10,"max":15},"maxColorDifference":1.5,"maxDeformation":1,"allowSurfaceDefects":false}}`)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/projects", nil).WithContext(ctx)
	started := make(chan struct{})
	req.Body = io.NopCloser(&observedBody{reader: bytes.NewReader(payload), started: started})
	recorder := httptest.NewRecorder()
	requestDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, req)
		close(requestDone)
	}()
	<-started
	cancel()
	close(releaseLock)
	if err = <-lockDone; err != nil {
		t.Fatal(err)
	}
	<-requestDone

	projects, err := service.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("请求 context 已取消后仍提交了 %d 个课题", len(projects))
	}
}
