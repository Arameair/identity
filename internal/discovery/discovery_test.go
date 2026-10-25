package discovery

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, h http.HandlerFunc) string {
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return strings.TrimPrefix(s.URL, "http://")
}

func TestParseAddress(t *testing.T) {
	ok := map[string]string{"127.0.0.1:7860": "127.0.0.1:7860", "http://localhost:7860/": "127.0.0.1:7860", " [::1]:7860 ": "[::1]:7860"}
	for in, want := range ok {
		if got, err := ParseAddress(in, 7798); err != nil || got != want {
			t.Errorf("%q -> %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"192.168.1.5:7860", "example.com:80", "127.0.0.1", "127.0.0.1:7798", "http://u:p@127.0.0.1:7860", "127.0.0.1:7860/#code=x", "https://127.0.0.1:7860", "127.0.0.1:99999"} {
		if _, err := ParseAddress(bad, 7798); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestRealShapedProcessorIsOnlyUnverified(t *testing.T) {
	var sawCookie, sawAuth bool
	addr := serve(t, func(w http.ResponseWriter, r *http.Request) {
		sawCookie = r.Header.Get("Cookie") != ""
		sawAuth = r.Header.Get("Authorization") != ""
		if r.URL.Path != "/api/health" {
			t.Errorf("probe touched %s", r.URL.Path)
		}
		w.Write([]byte(`{"app":"thought-processor","protocol":"thought-processor.diagnostic/0-experimental","version":"0.1.0-experiment"}`))
	})
	r := Probe(context.Background(), addr)
	if r.State != Detected || r.OpenURL != "http://"+addr+"/" || strings.Contains(strings.ToLower(r.State), "verified ") {
		t.Fatalf("%+v", r)
	}
	if sawCookie || sawAuth {
		t.Fatal("probe sent credentials")
	}
}

func TestWrongServiceAndOffline(t *testing.T) {
	ident := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"app":"identity","version":"x"}`)) })
	if r := Probe(context.Background(), ident); r.State != WrongService || r.OpenURL != "" {
		t.Fatalf("identity-as-processor: %+v", r)
	}
	html := serve(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	if r := Probe(context.Background(), html); r.State != Unrecognized {
		t.Fatalf("404 service: %+v", r)
	}
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()
	if r := Probe(context.Background(), addr); r.State != NotRunning {
		t.Fatalf("closed port: %+v", r)
	}
	if r := Probe(context.Background(), ""); r.State != NotConfigured {
		t.Fatalf("empty: %+v", r)
	}
}

func TestWatcherReportsTransitionsOnly(t *testing.T) {
	addr := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"app":"thought-processor","protocol":"thought-processor.diagnostic/0","version":"1"}`))
	})
	n := 0
	w := NewWatcher(addr, func(prev, cur Result) { n++ })
	for i := 0; i < 5; i++ {
		w.CheckNow(context.Background())
	}
	if n != 1 {
		t.Fatalf("expected one transition, got %d", n)
	}
}
