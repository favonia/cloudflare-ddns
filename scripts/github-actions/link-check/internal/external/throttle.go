package external

import (
	"net/url"
	"sync"
	"time"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
)

type hostThrottle struct {
	semaphores  map[string]chan struct{}
	mu          sync.Mutex
	lastRequest map[string]time.Time
	delay       time.Duration
}

func newHostThrottle(urls []extract.ExternalLink, maxPerHost int, delay time.Duration) *hostThrottle {
	capacity := max(maxPerHost, 1)
	semaphores := make(map[string]chan struct{})
	for _, link := range urls {
		host := hostFromURL(link.URL)
		if _, ok := semaphores[host]; !ok {
			semaphores[host] = make(chan struct{}, capacity)
		}
	}
	return &hostThrottle{
		semaphores:  semaphores,
		lastRequest: make(map[string]time.Time),
		delay:       delay,
	}
}

func (t *hostThrottle) acquire(host string) {
	if sem, ok := t.semaphores[host]; ok {
		sem <- struct{}{}
	}
	if t.delay > 0 {
		t.mu.Lock()
		elapsed := time.Since(t.lastRequest[host])
		t.mu.Unlock()
		if remaining := t.delay - elapsed; remaining > 0 {
			time.Sleep(remaining)
		}
	}
}

func (t *hostThrottle) release(host string) {
	if t.delay > 0 {
		t.mu.Lock()
		t.lastRequest[host] = time.Now()
		t.mu.Unlock()
	}
	if sem, ok := t.semaphores[host]; ok {
		<-sem
	}
}

func hostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return parsed.Host
}

func countUniqueHosts(urls []extract.ExternalLink) int {
	hosts := make(map[string]bool)
	for _, link := range urls {
		hosts[hostFromURL(link.URL)] = true
	}
	return len(hosts)
}
