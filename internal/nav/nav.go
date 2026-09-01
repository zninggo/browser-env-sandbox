// Package nav implements BES real top-level navigation: the redirect-following
// machinery that turns a single URL into a landed page while stamping request
// cookies per hop. It is kept free of V8/netlayer types so its behavior can be
// unit-tested without booting a live V8 isolate (the sandbox wires it via
// FollowRedirects with its own request/header closures).
package nav

import (
	"net/url"
	"strings"
)

// MaxHops caps how many 3xx redirects a single top-level navigation will follow
// before stopping (real browsers cap around 20; 10 is ample for SSO chains and
// bounds runaway redirect loops).
const MaxHops = 10

// HopResult is the per-hop outcome of a DoFunc call.
type HopResult struct {
	Status     int
	Headers    map[string]string // response headers, incl. optional Location
	SetCookies []string          // raw Set-Cookie lines this hop returned
}

// DoFunc issues one HTTP request for a single hop of the navigation. The
// wrapped requester is expected to stamp the hop's Set-Cookie onto whatever
// jar it manages (the sandbox applies cookies per hop inside its closure).
type DoFunc func(method, url string, headers map[string]string) (HopResult, error)

// headerLookup does a case-insensitive lookup of a header in a map whose keys
// may be in any case.
func headerLookup(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// FollowRedirects drives a single real top-level navigation: issues a GET at
// via do and follows the 3xx Location chain up to MaxHops hops, returning the
// landed URL, terminal status, hop count, and whether any redirect was followed.
// do is re-invoked per hop with descriptions re-derived by buildHeaders for the
// next hop's target (so cookie-jar scoping and Referer propagate hop to hop).
// Returned params: finalURL, terminal status, hop count, redirectUsed, error.
func FollowRedirects(
	do DoFunc,
	startURL string,
	buildHeaders func(target string) map[string]string,
	referer string,
) (finalURL string, status int, hops int, redirected bool, err error) {
	target := startURL
	hops = 0
	redirected = false
	for {
		hdrs := buildHeaders(target)
		if prev := referer; prev != "" && prev != target {
			if _, has := headerLookup(hdrs, "Referer"); !has {
				hdrs["Referer"] = prev
			}
		}
		res, rerr := do("GET", target, hdrs)
		if rerr != nil {
			return target, 0, hops, redirected, rerr
		}
		if res.Status >= 300 && res.Status < 400 && res.Status != 304 {
			loc := ""
			if v, ok := headerLookup(res.Headers, "Location"); ok {
				loc = v
			}
			if loc != "" && hops < MaxHops {
				if b, berr := url.Parse(target); berr == nil {
					if ref, rerr := url.Parse(loc); rerr == nil {
						target = b.ResolveReference(ref).String()
						hops++
						redirected = true
						continue
					}
				}
			}
		}
		return target, res.Status, hops, redirected, nil
	}
}
