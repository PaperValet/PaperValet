package loader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
	if _, en := Explain(ErrBadFile); !strings.Contains(en, "Linux .so") {
		t.Fatalf("bad file: %q", en)
	}
	if _, en := Explain(ErrNeedsRestart); !strings.Contains(en, "restart") {
		t.Fatalf("needs restart: %q", en)
	}
	zh, _ = Explain(&NameMismatchError{File: "foo", Plugin: "bar"})
	if !strings.Contains(zh, "bar") || !strings.Contains(zh, "bar.so") {
		t.Fatalf("mismatch: %q", zh)
	}
	_, en = Explain(errors.New("plugin.Open: plugin already loaded"))
	if !strings.Contains(en, "restart") {
		t.Fatalf("already loaded: %q", en)
	}
}

// soFixture builds a minimal real plugin .so under its own module
// example.com/plugins/<mod> — the same one-module-per-plugin shape as the
// plugin repository, so the buildinfo main path ends in /<mod>.
func soFixture(t *testing.T, dir, mod string) string {
	t.Helper()
	sub := filepath.Join(dir, "src-"+mod)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(sub, "go.mod"):  "module example.com/plugins/" + mod + "\n\ngo 1.25\n",
		filepath.Join(sub, "main.go"): "package main\n\nfunc main() {}\n",
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, mod+".so")
	cmd := exec.Command("go", "build", "-trimpath", "-buildmode=plugin", "-o", out, ".")
	cmd.Dir = sub
	cmd.Env = append(os.Environ(), "GO111MODULE=on", "GOFLAGS=", "GOTOOLCHAIN=auto")
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build plugin fixture: %v: %s", err, outb)
	}
	return out
}

func TestInspectFileRejectsRubbish(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.so")
	os.WriteFile(junk, []byte("not an elf at all"), 0o644)
	if _, _, err := inspectFile(junk); !errors.Is(err, ErrBadFile) {
		t.Fatalf("junk: %v", err)
	}
	elf := filepath.Join(dir, "elf.so")
	os.WriteFile(elf, append([]byte("\x7fELF\x02\x01\x01\x00"), []byte("truncated")...), 0o644)
	if _, _, err := inspectFile(elf); !errors.Is(err, ErrBadFile) {
		t.Fatalf("truncated elf: %v", err)
	}
}

func TestInspectFileReadsModuleName(t *testing.T) {
	dir := t.TempDir()
	path := soFixture(t, dir, "weather")
	name, digest, err := inspectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if name != "weather" {
		t.Fatalf("name = %q", name)
	}
	if len(digest) != 64 {
		t.Fatalf("digest = %q", digest)
	}
	// A second identical read yields the same digest.
	_, again, _ := inspectFile(path)
	if again != digest {
		t.Fatal("digest unstable")
	}
}

func TestInstallFileStagesUnderModuleName(t *testing.T) {
	dir := t.TempDir()
	src := soFixture(t, dir, "zeta")
	l := NewLoader(t.TempDir(), nil)

	// The loader cannot open the archive as a plugin, but the staging and
	// the swap detection can be observed through the digest bookkeeping.
	name, digest, err := inspectFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.stageFrom(src, name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l.dir, "zeta.so")); err != nil {
		t.Fatal(err)
	}
	l.mu.Lock()
	l.digests[name] = digest
	l.mu.Unlock()
	if got := l.knownDigest(name); got != digest {
		t.Fatalf("knownDigest = %q", got)
	}
	// Staging again does not disturb the digest.
	if err := l.stageFrom(src, name); err != nil {
		t.Fatal(err)
	}
	if l.knownDigest(name) != digest {
		t.Fatal("digest lost after restage")
	}
}

func TestFileSwappedDetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.so")
	os.WriteFile(path, []byte("\x7fELF mark-one"), 0o644)
	l := NewLoader(dir, nil)
	d1, _ := fileDigest(path)
	l.mu.Lock()
	l.digests["demo"] = d1
	l.mu.Unlock()

	if stale, _ := l.fileSwapped("demo", path); stale {
		t.Fatal("unchanged file reported stale")
	}
	os.WriteFile(path, []byte("\x7fELF mark-two"), 0o644)
	if stale, _ := l.fileSwapped("demo", path); !stale {
		t.Fatal("changed file not detected")
	}
}
