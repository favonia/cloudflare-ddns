package external

import (
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
)

func TestHostFromURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/path", "example.com"},
		{"https://example.com:8080/path", "example.com:8080"},
		{"http://sub.example.com", "sub.example.com"},
		{"not-a-url", "not-a-url"},
		{"https://", "https://"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := hostFromURL(tt.input)
			if got != tt.want {
				t.Errorf("hostFromURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHostThrottleConcurrencyLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const maxPerHost = 2
		urls := []extract.ExternalLink{
			{URL: "https://example.com/a"},
			{URL: "https://example.com/b"},
			{URL: "https://example.com/c"},
			{URL: "https://example.com/d"},
		}
		throttle := newHostThrottle(urls, maxPerHost, 0)

		var peak atomic.Int32
		var current atomic.Int32
		var wg sync.WaitGroup

		for range len(urls) {
			wg.Go(func() {
				host := "example.com"
				throttle.acquire(host)
				n := current.Add(1)
				for {
					old := peak.Load()
					if n <= old || peak.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(time.Second)
				current.Add(-1)
				throttle.release(host)
			})
		}
		wg.Wait()

		if got := peak.Load(); got > maxPerHost {
			t.Errorf("peak concurrent = %d, want <= %d", got, maxPerHost)
		}
	})
}

func TestHostThrottleDelay(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const delay = 500 * time.Millisecond
		urls := []extract.ExternalLink{
			{URL: "https://example.com/a"},
		}
		throttle := newHostThrottle(urls, 1, delay)
		host := "example.com"

		throttle.acquire(host)
		throttle.release(host)

		start := time.Now()
		throttle.acquire(host)
		elapsed := time.Since(start)
		throttle.release(host)

		if elapsed < delay {
			t.Errorf("delay between requests = %v, want >= %v", elapsed, delay)
		}
	})
}

func TestHostThrottleIndependentHosts(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const delay = 500 * time.Millisecond
		urls := []extract.ExternalLink{
			{URL: "https://a.example.com/x"},
			{URL: "https://b.example.com/y"},
		}
		throttle := newHostThrottle(urls, 1, delay)

		// First round: establish lastRequest timestamps for both hosts.
		for _, host := range []string{"a.example.com", "b.example.com"} {
			throttle.acquire(host)
			throttle.release(host)
		}

		// Second round: both hosts need to wait for the delay. If hosts
		// were independent, both delays run in parallel and the total
		// wall time equals one delay period, not two.
		start := time.Now()
		var wg sync.WaitGroup
		for _, host := range []string{"a.example.com", "b.example.com"} {
			wg.Go(func() {
				throttle.acquire(host)
				throttle.release(host)
			})
		}
		wg.Wait()

		elapsed := time.Since(start)
		if elapsed < delay {
			t.Errorf("elapsed = %v, want >= %v (one delay period)", elapsed, delay)
		}
		if elapsed >= 2*delay {
			t.Errorf("elapsed = %v, want < %v (hosts should delay in parallel)", elapsed, 2*delay)
		}
	})
}

func TestCountUniqueHosts(t *testing.T) {
	t.Parallel()
	urls := []extract.ExternalLink{
		{URL: "https://a.example.com/1"},
		{URL: "https://a.example.com/2"},
		{URL: "https://b.example.com/1"},
	}
	if got := countUniqueHosts(urls); got != 2 {
		t.Errorf("countUniqueHosts = %d, want 2", got)
	}
}
