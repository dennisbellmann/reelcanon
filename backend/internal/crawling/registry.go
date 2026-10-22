package crawling

import (
	"errors"
	"fmt"
	"sync"
)

var ErrCrawlerNotFound = errors.New("crawler not registered")

type Registry struct {
	mu       sync.RWMutex
	crawlers map[string]Crawler
}

func NewRegistry() *Registry {
	return &Registry{crawlers: make(map[string]Crawler)}
}

func (r *Registry) Register(crawler Crawler) error {
	if crawler == nil || crawler.Key() == "" {
		return errors.New("crawler and non-empty crawler key are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.crawlers[crawler.Key()]; exists {
		return fmt.Errorf("crawler %q is already registered", crawler.Key())
	}
	r.crawlers[crawler.Key()] = crawler
	return nil
}

func (r *Registry) Get(key string) (Crawler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	crawler, exists := r.crawlers[key]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrCrawlerNotFound, key)
	}
	return crawler, nil
}
