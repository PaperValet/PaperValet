package loader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestValidName(t *testing.T) {
	for _, n := range []string{"weather", "gt", "a_b-c", "X1"} {
		if !ValidName(n) {
			t.Errorf("%q rejected", n)
		}
	}
	for _, n := range []string{"", "../x", "a/b", "a.so", strings.Repeat("a", 33), "a b"} {
		if ValidName(n) {
			t.Errorf("%q accepted", n)
		}
	}
}

func newRepo(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		switch r.URL.Path {
		case "/plugins.json":
			_, _ = w.Write([]byte(`[{"name":"zeta","version":"1"},{"name":"../evil"},{"name":"alpha","description":"A"}]`))
		case "/demo.so":
			_, _ = w.Write([]byte("ELF"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInstallDownloads(t *testing.T) {
	srv := newRepo(t, nil)
	dir := t.TempDir()
	l := NewLoader(dir, nil)
	l.SetRepoURL(srv.URL + "/")
	ctx := context.Background()

	if err := l.Install(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "demo.so")); string(b) != "ELF" {
		t.Fatalf("content = %q", b)
	}
	if err := l.Install(ctx, "demo"); !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("second install: %v", err)
	}
	if err := l.Install(ctx, "missing"); !errors.Is(err, ErrNotInRepo) {
		t.Fatalf("missing: %v", err)
	}
	if err := l.Install(ctx, "../x"); !errors.Is(err, ErrBadName) {
		t.Fatalf("traversal: %v", err)
	}
	// No partial files are left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			t.Fatalf("leftover %s", e.Name())
		}
	}
	if err := l.Remove(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if err := l.Remove(ctx, "demo"); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("second remove: %v", err)
	}
}

func TestIndexSortsFiltersAndCaches(t *testing.T) {
	var hits int32
	srv := newRepo(t, &hits)
	l := NewLoader(t.TempDir(), nil)
	l.SetRepoURL(srv.URL)
	ctx := context.Background()

	got, err := l.Index(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Fatalf("index = %+v", got)
	}
	got[0].Name = "mutated"
	again, _ := l.Index(ctx, false)
	if again[0].Name != "alpha" || atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("cache: %+v hits=%d", again, hits)
	}
	if _, err := l.Index(ctx, true); err != nil || atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("fresh fetch: %v hits=%d", err, hits)
	}
}

func TestFailedTracksBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.so")
	if err := os.WriteFile(path, []byte("not elf"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := NewLoader(dir, nil)
	_ = l.LoadAll(context.Background())
	if _, ok := l.Failed()["broken"]; !ok {
		t.Fatalf("failed = %v", l.Failed())
	}
	os.Remove(path)
	if len(l.Failed()) != 0 {
		t.Fatal("deleted file still reported")
	}
}

func TestExplain(t *testing.T) {
	zh, en := Explain(errors.New("plugin.Open: plugin was built with a different version of package x"))
	if !strings.Contains(zh, "update -f") || !strings.Contains(en, "update -f") {
		t.Fatalf("version mismatch: %q %q", zh, en)
	}
	if zh, _ := Explain(ErrNotInRepo); zh != "仓库里没有这个插件" {
		t.Fatalf("not in repo: %q", zh)
	}
	if zh, en := Explain(errors.New("raw")); zh != "raw" || en != "raw" {
		t.Fatal("raw passthrough")
	}
}
