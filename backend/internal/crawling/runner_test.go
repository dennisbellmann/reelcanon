package crawling

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeCrawler struct {
	fetch    func(context.Context) (SourceDocument, error)
	snapshot Snapshot
}

func (fakeCrawler) Key() string { return "fixture" }
func (c fakeCrawler) Fetch(ctx context.Context, _ string) (SourceDocument, error) {
	return c.fetch(ctx)
}
func (c fakeCrawler) Parse(SourceDocument) (Snapshot, error) { return c.snapshot, nil }
func TestRunnerRetriesAndReleasesLockAfterFailure(t *testing.T) {
	var attempts atomic.Int32
	registry := NewRegistry()
	crawler := fakeCrawler{snapshot: snapshotWithCount("fixture", 3), fetch: func(context.Context) (SourceDocument, error) {
		if attempts.Add(1) < 3 {
			return SourceDocument{}, errors.New("temporary failure")
		}
		return SourceDocument{}, nil
	}}
	if err := registry.Register(crawler); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(crawler); err == nil {
		t.Fatal("duplicate registration accepted")
	}
	runner := NewRunner(registry, 2)
	if _, err := runner.Update(context.Background(), "fixture", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 3 {
		t.Fatal("wrong retry count")
	}
	if _, err := runner.Update(context.Background(), "fixture", "https://example.com"); err != nil {
		t.Fatal("lock not released")
	}
}
func TestRunnerPreventsOverlapAndRespectsCancellation(t *testing.T) {
	started := make(chan struct{})
	registry := NewRegistry()
	if err := registry.Register(fakeCrawler{snapshot: snapshotWithCount("fixture", 3), fetch: func(ctx context.Context) (SourceDocument, error) {
		close(started)
		<-ctx.Done()
		return SourceDocument{}, ctx.Err()
	}}); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(registry, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := runner.Update(ctx, "fixture", "https://example.com"); done <- err }()
	<-started
	if _, err := runner.Update(context.Background(), "fixture", "https://example.com"); !errors.Is(err, ErrUpdateInProgress) {
		t.Fatalf("overlap accepted: %v", err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation ignored")
	}
}
