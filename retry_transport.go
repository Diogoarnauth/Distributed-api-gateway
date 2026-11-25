// retry_transport.go: picks the upstream for each attempt and retries safely.
//
// Each route has ONE reverse proxy whose Transport is a retryTransport. The
// proxy hands it the outgoing request without a destination; for every
// attempt the retryTransport asks the route's round-robin balancer for the
// next upstream, points the request there and sends it through the real
// http.Transport (transport.go), which still applies the timeouts.
//
//	proxy -> retryTransport.RoundTrip
//	           attempt 1: balancer -> 8082 -> 503        (proxy never sees it)
//	           retry.ShouldRetry? yes -> short random wait (backoff + jitter)
//	           attempt 2: balancer -> 8081 -> 200
//	        <- only the final result goes back to the proxy and the client
//
// Retries must happen here, before the proxy gets a response: once the proxy
// starts writing to the client, the status can't be taken back.
//
// The request body can only be read once, so it is kept in memory (up to
// maxRetryBody) to send it again. A bigger body is streamed once, no retries.
// Whether a failed attempt may be retried is decided by internal/retry: the
// policy (is it safe?) and the route's budget (are retries within ~10% of
// the traffic?).

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"distributed-api-gateway/internal/balancer"
	"distributed-api-gateway/internal/retry"
)

const (
	// maxAttempts counts the first try: 2 means at most one retry, so a
	// failing upstream gets at most 2x the traffic.
	maxAttempts = 2

	// maxRetryBody is the largest request body kept in memory for retries.
	maxRetryBody = 1 << 20 // 1 MiB

	// Wait before a retry: a random value in [min, max) (jitter), so many
	// requests failing together don't all retry at the same instant. Short,
	// because the retry goes to a different upstream.
	retryBackoffMin = 25 * time.Millisecond
	retryBackoffMax = 100 * time.Millisecond

	// Retry budget (internal/retry/budget.go): retries may use at most ~10%
	// of the route's traffic, with up to 10 saved-up retries for bursts.
	retryBudgetRatio = 0.1
	retryBudgetMax   = 10
)

type retryTransport struct {
	upstreams []*url.URL
	balancer  *balancer.RoundRobin
	budget    *retry.Budget     // per route: one route's failures don't spend another's
	next      http.RoundTripper // the real http.Transport with the timeouts
}

func newRetryTransport(upstreams []*url.URL, next http.RoundTripper) *retryTransport {
	return &retryTransport{
		upstreams: upstreams,
		balancer:  balancer.NewRoundRobin(),
		budget:    retry.NewBudget(retryBudgetRatio, retryBudgetMax),
		next:      next,
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Every client request earns the route a fraction of a retry.
	t.budget.OnRequest()

	body, retryable, err := prepareBody(req)
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}

	attempts := maxAttempts
	if !retryable {
		attempts = 1
	}

	hasKey := req.Header.Get("Idempotency-Key") != ""

	for attempt := 1; ; attempt++ {
		target := t.upstreams[t.balancer.Next(len(t.upstreams))]

		// A RoundTripper must not modify the request it receives, so each
		// attempt works on its own copy.
		out := req.Clone(req.Context())
		pointTo(out, target)

		if body != nil {
			out.Body = body()
		}

		resp, err := t.next.RoundTrip(out)

		outcome := retry.Attempt{
			Method:            req.Method,
			HasIdempotencyKey: hasKey,
			Err:               err,
		}

		if err != nil {
			// Keep the cause (errors.Is/As still work) and add which
			// upstream failed, for the proxy's logs.
			err = fmt.Errorf("upstream %s: %w", target.Host, err)
		} else {
			outcome.StatusCode = resp.StatusCode
		}

		if attempt >= attempts {
			return resp, err
		}

		ok, reason := retry.ShouldRetry(outcome)
		if !ok {
			return resp, err
		}

		// Safe to retry, but is there budget? If many requests are failing
		// at once, retrying them all would only add load: give up instead.
		if !t.budget.TryRetry() {
			log.Printf(
				"Not retrying %s %s: retry budget exhausted (attempt %d to %s failed: %s)",
				req.Method, req.URL.Path, attempt, target.Host, reason,
			)
			return resp, err
		}

		log.Printf(
			"Retrying %s %s: attempt %d to %s failed (%s)",
			req.Method, req.URL.Path, attempt, target.Host, reason,
		)

		if resp != nil {
			discard(resp)
		}

		if err := waitBackoff(req.Context()); err != nil {
			// Client left or total budget ran out while waiting.
			return nil, err
		}
	}
}

// prepareBody makes the request body sendable more than once. It returns a
// function giving a fresh body for each attempt (nil when there is no body)
// and whether retries are possible.
func prepareBody(req *http.Request) (func() io.ReadCloser, bool, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, true, nil
	}

	// Read at most maxRetryBody+1 bytes: one extra byte tells us whether
	// the body is bigger than the limit.
	buf, err := io.ReadAll(io.LimitReader(req.Body, maxRetryBody+1))
	if err != nil {
		req.Body.Close()
		return nil, false, err
	}

	if len(buf) > maxRetryBody {
		// Too big to keep: send what we read followed by the rest of the
		// stream, once.
		rest := req.Body
		return func() io.ReadCloser {
			return struct {
				io.Reader
				io.Closer
			}{io.MultiReader(bytes.NewReader(buf), rest), rest}
		}, false, nil
	}

	req.Body.Close()

	return func() io.ReadCloser {
		return io.NopCloser(bytes.NewReader(buf))
	}, true, nil
}

// pointTo sends the request to target, like ProxyRequest.SetURL: scheme and
// host of the upstream, the upstream's base path (if any) before the request
// path, and an empty Host so the Host header matches the upstream.
func pointTo(out *http.Request, target *url.URL) {
	out.URL.Scheme = target.Scheme
	out.URL.Host = target.Host
	out.URL.Path = joinPath(target.Path, out.URL.Path)
	out.URL.RawPath = ""
	out.Host = ""
}

func joinPath(base, path string) string {
	if base == "" || base == "/" {
		return path
	}

	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
}

// discard drains and closes a response we won't use, so its connection can
// go back to the pool instead of being closed.
func discard(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
}

// waitBackoff sleeps a random time in [retryBackoffMin, retryBackoffMax),
// but returns early with the context's error if it ends first.
func waitBackoff(ctx context.Context) error {
	wait := retryBackoffMin + rand.N(retryBackoffMax-retryBackoffMin)

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
