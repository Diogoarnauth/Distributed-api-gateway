// Package retry decides when it is safe to retry a request to an upstream.
//
// A retry can hide a passing failure from the client (another upstream may
// work), but it can also run the same operation twice: a timeout does not
// mean the upstream did nothing. So the rule is: retry only when we KNOW the
// upstream never got the request, or when running it twice is harmless.
//
//	what happened                                   retry?
//	client canceled / total time budget ran out     never
//	could not connect (request never sent)          yes, any method
//	timeout, connection cut, upstream 502/503/504   only if idempotent
//	any other status (2xx, 3xx, 4xx, 500, ...)      never
//
// "Idempotent" means running it twice leaves the same final state: GET,
// HEAD, OPTIONS, TRACE, PUT and DELETE by HTTP's definition, or any request
// that carries an Idempotency-Key (the upstream deduplicates it).
//
// This file only holds the rules; the retry loop asks ShouldRetry after
// every failed attempt.
package retry

import (
	"context"
	"errors"
	"net"
	"net/http"
)

// Attempt describes the outcome of one try to reach an upstream.
type Attempt struct {
	Method            string
	HasIdempotencyKey bool

	// Err is the Transport error when no response arrived; nil otherwise.
	Err error

	// StatusCode is the upstream's status when a response arrived (Err == nil).
	StatusCode int
}

// ShouldRetry reports whether the request may be sent again, and why.
// The reason is meant for logs.
func ShouldRetry(a Attempt) (bool, string) {
	if a.Err != nil {
		return shouldRetryError(a)
	}

	return shouldRetryStatus(a)
}

func shouldRetryError(a Attempt) (bool, string) {
	// Our own context ended: the client left or the total budget ran out.
	// Nobody is waiting for a retry.
	if errors.Is(a.Err, context.Canceled) {
		return false, "client canceled"
	}

	if errors.Is(a.Err, context.DeadlineExceeded) {
		return false, "total time budget exceeded"
	}

	// Never connected: the upstream did not receive anything, so even a
	// POST is safe to send again.
	if neverSent(a.Err) {
		return true, "connection failed, request never sent"
	}

	// Anything else (response timeout, connection reset, unexpected EOF)
	// happened after the request may have reached the upstream.
	if isIdempotent(a) {
		return true, "ambiguous failure, but request is idempotent"
	}

	return false, "ambiguous failure and request is not idempotent"
}

func shouldRetryStatus(a Attempt) (bool, string) {
	switch a.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		// The upstream (or something behind it) says "can't right now".
		// It may still have done part of the work, so only if idempotent.
		if isIdempotent(a) {
			return true, "upstream unavailable, request is idempotent"
		}

		return false, "upstream unavailable, but request is not idempotent"
	}

	// Success, redirects, client errors (4xx) and 500 (the upstream processed
	// it and failed; retrying will most likely fail the same way).
	return false, "status is not retryable"
}

// neverSent reports whether err happened while connecting (DNS lookup or
// TCP dial), i.e. before any byte of the request left the gateway.
func neverSent(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}

	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func isIdempotent(a Attempt) bool {
	if a.HasIdempotencyKey {
		return true
	}

	switch a.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace,
		http.MethodPut, http.MethodDelete:
		return true
	}

	return false
}
