package nav

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// fakeJar is a tiny host-scoped cookie jar mirroring the sandbox CookieStore
// semantics used by nav's caller: request cookies are attached per host, and
// every response's Set-Cookie is stamped per host.
type fakeJar struct {
	mu    sync.Mutex
	cooks map[string]map[string]string // host -> name -> value
	seen  []string                     // outbound "host Cookie: value" per hop
}

func (j *fakeJar) hostOf(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// buildHeaders + plain-equivalent of the sandbox navHeaderFactory: attaches the
// jar cookies scoped to target's host.
func (j *fakeJar) buildHeaders(target string) map[string]string {
	hdrs := make(map[string]string)
	h := j.hostOf(target)
	if h == "" {
		return hdrs
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	parts := []string{}
	for n, v := range j.cooks[h] {
		parts = append(parts, n+"="+v)
	}
	if len(parts) > 0 {
		hdrs["Cookie"] = strings.Join(parts, "; ")
	}
	return hdrs
}

// do issues one hop against the server (no redirect following) and stamps the
// hop's Set-Cookie into the jar for the hop host, like the sandbox wrapper.
func (j *fakeJar) do(server *httptest.Server) DoFunc {
	return func(method, target string, headers map[string]string) (HopResult, error) {
		h := j.hostOf(target)
		j.mu.Lock()
		j.seen = append(j.seen, fmt.Sprintf("%s:%s:cookie=%s", h, target, headers["Cookie"]))
		j.mu.Unlock()

		req, err := http.NewRequest(method, target, nil)
		if err != nil {
			return HopResult{}, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := cl.Do(req)
		if err != nil {
			return HopResult{}, err
		}
		defer resp.Body.Close()

		hdrs := map[string]string{}
		for k, vs := range resp.Header {
			if len(vs) > 0 {
				hdrs[k] = vs[0]
			}
		}
		var setCookies []string
		for _, sc := range resp.Header["Set-Cookie"] {
			setCookies = append(setCookies, sc)
			kv := strings.SplitN(sc, ";", 2)[0]
			nv := strings.SplitN(kv, "=", 2)
			if len(nv) == 2 {
				j.mu.Lock()
				if j.cooks[h] == nil {
					j.cooks[h] = map[string]string{}
				}
				j.cooks[h][strings.TrimSpace(nv[0])] = nv[1]
				j.mu.Unlock()
			}
		}
		return HopResult{Status: resp.StatusCode, Headers: hdrs, SetCookies: setCookies}, nil
	}
}

func jarHas(j *fakeJar, host, name, want string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	v, ok := j.cooks[host][name]
	return ok && v == want
}

func TestFollowRedirectsSameHostChain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "a", Value: "1"})
		w.Header().Set("Location", "/b")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "b", Value: "2"})
		w.Header().Set("Location", "/c")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "c", Value: "3"})
		w.Write([]byte("landed"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar := &fakeJar{cooks: map[string]map[string]string{}}
	final, status, hops, redirected, err := FollowRedirects(jar.do(srv), srv.URL+"/a", jar.buildHeaders, srv.URL)
	if err != nil {
		t.Fatalf("FollowRedirects err: %v", err)
	}
	if final != srv.URL+"/c" {
		t.Errorf("final URL = %s, want %s", final, srv.URL+"/c")
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if hops != 2 {
		t.Errorf("hops = %d, want 2", hops)
	}
	if !redirected {
		t.Errorf("redirected = false, want true")
	}
	host := jar.hostOf(srv.URL + "/a")
	for _, n := range []string{"a", "b", "c"} {
		if !jarHas(jar, host, n, map[string]string{"a": "1", "b": "2", "c": "3"}[n]) {
			t.Errorf("jar missing cookie %s on %s", n, host)
		}
	}
	// Prove cookie carry-over per hop: the hop to /b carried the /a cookie.
	found := false
	for _, s := range jar.seen {
		if strings.Contains(s, "/b:") && strings.Contains(s, "a=1") {
			found = true
		}
	}
	if !found {
		t.Errorf("hop /b did not carry cookie a=1 as request header: %v", jar.seen)
	}
}

// TestFollowRedirectsAbsoluteLocationCrossOrigin verifies a redirect with an
// absolute Location (different origin) is followed, its 200 lands and its
// Set-Cookie is stamped on the landing host.
func TestFollowRedirectsAbsoluteLocationCrossOrigin(t *testing.T) {
	var srv2 *httptest.Server
	srv2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "land", Value: "2"})
		w.Write([]byte("landed"))
	}))
	defer srv2.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "first", Value: "1"})
		w.Header().Set("Location", srv2.URL)
		w.WriteHeader(302)
	})
	srv1 := httptest.NewServer(mux)
	defer srv1.Close()

	jar := &fakeJar{cooks: map[string]map[string]string{}}
	final, status, hops, redirected, err := FollowRedirects(jar.do(srv1), srv1.URL+"/a", jar.buildHeaders, "")
	if err != nil {
		t.Fatalf("FollowRedirects err: %v", err)
	}
	if final != srv2.URL {
		t.Errorf("final = %s, want %s", final, srv2.URL)
	}
	if status != 200 || hops != 1 || !redirected {
		t.Errorf("got status=%d hops=%d redirected=%v", status, hops, redirected)
	}
	h1 := jar.hostOf(srv1.URL)
	h2 := jar.hostOf(srv2.URL)
	if h1 != h2 { // distinct hostnames would each own their cookies
		if !jarHas(jar, h1, "first", "1") || !jarHas(jar, h2, "land", "2") {
			t.Errorf("jar wrong: h1=%v h2=%v", jar.cooks[h1], jar.cooks[h2])
		}
	} else { // same 127.0.0.1 hostname (RFC: port-independent) — both land there
		if !jarHas(jar, h1, "first", "1") || !jarHas(jar, h1, "land", "2") {
			t.Errorf("jar wrong on shared hostname %s: %v", h1, jar.cooks[h1])
		}
	}
}

// TestBuildHeadersHostScoping proves the jar hands out cookies ONLY for the
// request host (the sandbox CookieHeaderFor contract), using distinct
// hostnames so cross-host isolation is unambiguous without network.
func TestBuildHeadersHostScoping(t *testing.T) {
	jar := &fakeJar{cooks: map[string]map[string]string{
		"host-a.example": {"sid": "AAA"},
		"host-b.example": {"sid": "BBB"},
	}}
	hdrsA := jar.buildHeaders("https://host-a.example/path")
	if hdrsA["Cookie"] != "sid=AAA" {
		t.Errorf("host-a cookie = %q, want sid=AAA", hdrsA["Cookie"])
	}
	hdrsB := jar.buildHeaders("https://host-b.example/x")
	if hdrsB["Cookie"] != "sid=BBB" {
		t.Errorf("host-b cookie = %q, want sid=BBB", hdrsB["Cookie"])
	}
	hdrsC := jar.buildHeaders("https://unrelated.example/")
	if _, ok := hdrsC["Cookie"]; ok {
		t.Errorf("unrelated host leaked cookie %q", hdrsC["Cookie"])
	}
}

// TestFollowRedirectsCap verifies a runaway redirect loop stops at MaxHops.
func TestFollowRedirectsCap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/n")
		w.WriteHeader(302)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar := &fakeJar{cooks: map[string]map[string]string{}}
	final, status, hops, redirected, err := FollowRedirects(jar.do(srv), srv.URL+"/a", jar.buildHeaders, "")
	if err != nil {
		t.Fatalf("FollowRedirects err: %v", err)
	}
	if hops != MaxHops {
		t.Errorf("hops = %d, want cap %d", hops, MaxHops)
	}
	if status != 302 {
		t.Errorf("terminal status = %d, want 302 (cap reached mid-chain)", status)
	}
	if !redirected {
		t.Errorf("redirected = false, want true")
	}
	_ = final
}

// TestFollowRedirectsImmediate200 verifies a non-redirect response returns
// untouched (hops=0, redirected=false).
func TestFollowRedirectsImmediate200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	jar := &fakeJar{cooks: map[string]map[string]string{}}
	final, status, hops, redirected, err := FollowRedirects(jar.do(srv), srv.URL+"/ok", jar.buildHeaders, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if final != srv.URL+"/ok" || status != 200 || hops != 0 || redirected {
		t.Errorf("got final=%s status=%d hops=%d redirected=%v", final, status, hops, redirected)
	}
}

func TestFollowRedirectsRelativeLocation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/rel")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/rel", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("rel"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	jar := &fakeJar{cooks: map[string]map[string]string{}}
	final, status, hops, _, err := FollowRedirects(jar.do(srv), srv.URL+"/a", jar.buildHeaders, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if final != srv.URL+"/rel" || status != 200 || hops != 1 {
		t.Errorf("got final=%s status=%d hops=%d", final, status, hops)
	}
}
