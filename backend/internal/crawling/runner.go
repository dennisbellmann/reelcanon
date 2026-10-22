package crawling

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrUpdateInProgress = errors.New("an update is already running for this crawler")

type Runner struct {
	registry *Registry
	mu       sync.Mutex
	active   map[string]struct{}
	retries  int
}

func NewRunner(registry *Registry, retries int) *Runner {
	if retries < 0 {
		retries = 0
	}
	if retries > 5 {
		retries = 5
	}
	return &Runner{registry: registry, active: make(map[string]struct{}), retries: retries}
}

// Update fetches and validates one Canon snapshot. Publication remains a
// separate call through the shared catalog importer after this returns.
func (r *Runner) Update(ctx context.Context, crawlerKey, sourceURL string) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	crawler, err := r.registry.Get(crawlerKey)
	if err != nil {
		return Snapshot{}, err
	}
	if err := r.acquire(crawlerKey); err != nil {
		return Snapshot{}, err
	}
	defer r.release(crawlerKey)

	var document SourceDocument
	for attempt := 0; attempt <= r.retries; attempt++ {
		document, err = crawler.Fetch(ctx, sourceURL)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return Snapshot{}, ctx.Err()
		}
		if attempt == r.retries {
			return Snapshot{}, fmt.Errorf("fetch %s after %d attempts: %w", crawlerKey, attempt+1, err)
		}
		wait := time.NewTimer(time.Duration(attempt+1) * 200 * time.Millisecond)
		select {
		case <-ctx.Done():
			wait.Stop()
			return Snapshot{}, ctx.Err()
		case <-wait.C:
		}
	}

	snapshot, err := crawler.Parse(document)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse %s snapshot: %w", crawlerKey, err)
	}
	if err := ValidateSourceSnapshot(snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("validate %s snapshot: %w", crawlerKey, err)
	}
	return snapshot, nil
}

func (r *Runner) acquire(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.active[key]; exists {
		return ErrUpdateInProgress
	}
	r.active[key] = struct{}{}
	return nil
}

func (r *Runner) release(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.active, key)
}
