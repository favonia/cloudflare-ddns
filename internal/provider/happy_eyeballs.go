package provider

import (
	"context"
	"net/netip"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// HappyEyeballsAlternativeDelay is the delay to start the alternative
// detection method.
const HappyEyeballsAlternativeDelay = time.Second / 2

// Hint1111BlocakageText is the explanation of why we want the parallel detection algorithm.
const Hint1111BlocakageText string = "Your IPv4 provider is using 1.1.1.1 to get your public IP. Sometimes, your ISP or router might block this. If that happens, we'll try 1.0.0.1 instead. Whichever works first will be used from then on." //nolint:lll

// Hint1111Blockage prints out a message that explains the parallel connection to 1.0.0.1.
func Hint1111Blockage(ppfmt pp.PP) { ppfmt.Hintf(pp.Hint1111Blockage, "%s", Hint1111BlocakageText) }

type splitResult struct {
	method protocol.Method
	ip     netip.Addr
	ok     bool
}

type happyEyeballs struct {
	provider SplitProvider
	chosen   map[ipnet.Type]protocol.Method
}

// NewHappyEyeballs creates a new [Provider] by applying the Happy Eyeballs algorithm to [SplitProvider].
func NewHappyEyeballs(provider SplitProvider) Provider {
	return happyEyeballs{
		provider: provider,
		chosen:   map[ipnet.Type]protocol.Method{},
	}
}

// Name calls the [SplitProvider.Name].
func (p happyEyeballs) Name() string {
	return p.provider.Name()
}

// GetIP initiates both methods simultaneously.
//
// This is basically the Happy Eyeballs, with two major changes:
//  1. The delay to initiate the alternative method is controlled
//     by [HappyEyeballsAlternativeDelay], which is longer than
//     what's recommended in RFC 6555.
//  2. The state will not be flushed because 1.1.1.1 is not
func (p happyEyeballs) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, protocol.Method, bool) {
	if !p.provider.HasAlternative(ipNet) {
		p.chosen[ipNet] = protocol.MethodPrimary
	}

	if method := p.chosen[ipNet]; method != protocol.MethodUnspecified {
		ip, ok := p.provider.GetIP(ctx, ppfmt, ipNet, method)
		return ip, method, ok
	}

	finished := make(chan struct{})
	defer close(finished)

	splitResults := make(chan splitResult)
	failed := map[protocol.Method]bool{}
	queuedPP := map[protocol.Method]pp.QueuedPP{}

	start := func(ctx context.Context, ppfmt pp.PP, method protocol.Method) {
		ip, ok := p.provider.GetIP(ctx, ppfmt, ipNet, method)

		select {
		case <-finished: // done
		case splitResults <- splitResult{method: method, ip: ip, ok: ok}:
		}
	}

	fork := func(method protocol.Method) func() {
		ctx, cancel := context.WithCancel(ctx)
		queuedPP[method] = pp.NewQueued(ppfmt)
		go start(ctx, queuedPP[method], method)

		return cancel
	}

	primaryCancel := fork(protocol.MethodPrimary)
	defer primaryCancel()

	// Some delay for the alternative method so that
	// 1. We prefer the primary method, and
	// 2. We are not making lots of requests at the same time.
	alternativeDelayTimer := time.NewTimer(HappyEyeballsAlternativeDelay)

	for {
		select {
		case <-alternativeDelayTimer.C:
			alternativeCancel := fork(protocol.MethodAlternative)
			defer alternativeCancel()

		case r := <-splitResults:
			if r.ok {
				// remember this successful IP detection
				p.chosen[ipNet] = r.method
				queuedPP[r.method].Flush()

				if r.method == protocol.MethodAlternative {
					ppfmt.Infof(pp.EmojiNow, "The server 1.0.0.1 responded before 1.1.1.1 does and will be used from now on.")
				}

				return r.ip, r.method, r.ok
			}

			// Record the failure.
			failed[r.method] = true

			// If both methods fail, then the detection fails.
			if failed[protocol.MethodPrimary] && failed[protocol.MethodAlternative] {
				// Flush out the messages from the primary method.
				queuedPP[protocol.MethodPrimary].Flush()

				return netip.Addr{}, protocol.MethodUnspecified, false
			}

			// If the primary method fails, start the alternative method immediately.
			if r.method == protocol.MethodPrimary {
				if alternativeDelayTimer.Stop() {
					alternativeDelayTimer.Reset(0)
				}
			}
		}
	}
}
