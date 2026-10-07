package loader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goplugin "plugin"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Errors callers branch on.
var (
	ErrAlreadyInstalled = errors.New("already installed")
	ErrNotInstalled     = errors.New("not installed")
	ErrNotInRepo        = errors.New("not in the repository")
	ErrBadName          = errors.New("invalid plugin name")
	// ErrBadFile is returned when a file to install is not a Linux plugin
	// .so (wrong magic).
	ErrBadFile = errors.New("not a plugin file")
	// ErrNeedsRestart is returned when new plugin bytes sit at a path this
	// process has already opened: a loaded Go module cannot be swapped, so
	// the file is kept and loads on the next start.
	ErrNeedsRestart = errors.New("plugin file already open in this process; restart to swap it")
)

// NameMismatchError is returned when a .so file's name differs from the
// Name() of the plugin inside it. Every loader entry — loaded, failed,
// settings, the panels — is keyed by file name, so the two must match.
type NameMismatchError struct {
	File   string
	Plugin string
}

func (e *NameMismatchError) Error() string {
	return fmt.Sprintf("plugin file %s.so contains plugin %q; the file must be named %s.so", e.File, e.Plugin, e.Plugin)
}

// nameRe is what a plugin file name may look like; it keeps names coming
// from chats and bot buttons out of other directories.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// ValidName reports whether name is a usable external plugin name.
func ValidName(name string) bool { return nameRe.MatchString(name) }

// Loader loads/unloads/manages external plugins from .so files. Every
// operation is serialized, so commands and bot taps can race freely.
type Loader struct {
	dir     string
	manager pkgplugin.Manager
	logger  pkgplugin.Logger
	http    *http.Client

	op sync.Mutex // serializes load/unload/install/remove

	mu      sync.RWMutex
	loaded  map[string]*LoadedPlugin
	failed  map[string]string
	digests map[string]string // file name -> sha256 of the bytes goplugin.Open saw
	repoURL string
	index   []IndexEntry
	indexAt time.Time
}

// LoadedPlugin represents a loaded .so plugin.
type LoadedPlugin struct {
	Plugin   pkgplugin.Plugin
	Handle   *goplugin.Plugin
	Path     string
	Metadata *pkgplugin.PluginMetadata
	LoadedAt time.Time
}

// IndexEntry is one plugin in the repository index (plugins.json).
type IndexEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DescEN      string `json:"desc_en,omitempty"`
	Version     string `json:"version"`
}

// NewLoader creates a new plugin loader.
func NewLoader(dir string, mgr pkgplugin.Manager) *Loader {
	return &Loader{
		dir:     dir,
		manager: mgr,
		loaded:  make(map[string]*LoadedPlugin),
		failed:  make(map[string]string),
		digests: make(map[string]string),
		logger:  logger.NamedLogger("plugin_loader"),
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
		repoURL: "https://github.com/PaperValet/PaperValet-Plugins/releases/latest/download",
	}
}

// LoadAll loads all .so plugins from the plugins directory. Files that
// fail are remembered (see Failed) instead of aborting the rest.
func (l *Loader) LoadAll(ctx context.Context) error {
	l.op.Lock()
	defer l.op.Unlock()
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Info("plugins directory does not exist, skipping", "dir", l.dir)
			return nil
		}
		return fmt.Errorf("read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}
		path := filepath.Join(l.dir, entry.Name())
		if err := l.load(ctx, path); err != nil {
			l.logger.Error("failed to load plugin", "path", path, "error", err)
			continue
		}
	}
	l.logger.Info("external plugins loaded", "count", len(l.GetLoaded()))
	return nil
}

// Load loads a single plugin from a .so file path.
func (l *Loader) Load(ctx context.Context, path string) error {
	l.op.Lock()
	defer l.op.Unlock()
	return l.load(ctx, path)
}

func (l *Loader) load(ctx context.Context, path string) (err error) {
	name := strings.TrimSuffix(filepath.Base(path), ".so")
	defer func() {
		l.mu.Lock()
		if err != nil && !errors.Is(err, ErrAlreadyInstalled) && !errors.Is(err, ErrNeedsRestart) {
			l.failed[name] = err.Error()
		} else {
			delete(l.failed, name)
		}
		l.mu.Unlock()
	}()

	if l.IsLoaded(name) {
		return fmt.Errorf("plugin %s: %w", name, ErrAlreadyInstalled)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin file not found: %s", path)
	}
	// A file this process already opened under different bytes: Go cannot
	// swap a loaded module, and re-opening the path would silently run the
	// old code. Keep the new bytes and load them on the next start.
	if stale, digest := l.fileSwapped(name, path); stale {
		l.mu.Lock()
		l.digests[name] = digest
		l.mu.Unlock()
		return ErrNeedsRestart
	}

	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin: %w", err)
	}

	newSymbol, err := p.Lookup("New")
	if err != nil {
		return fmt.Errorf("plugin missing New function: %w", err)
	}

	plug, err := instantiate(newSymbol)
	if err != nil {
		return err
	}
	// Everything in the loader, the panels and the settings store is keyed
	// by file name; a plugin that names itself differently would orphan its
	// settings and can shadow a built-in.
	if plug.Name() != name {
		return &NameMismatchError{File: name, Plugin: plug.Name()}
	}

	var meta *pkgplugin.PluginMetadata
	if metaSym, err := p.Lookup("Metadata"); err == nil {
		meta = metadataOf(metaSym)
	}

	if err := l.manager.RegisterPlugin(plug); err != nil {
		return fmt.Errorf("register plugin: %w", err)
	}
	if err := safeCall(func() error { return plug.Init(ctx, l.manager) }); err != nil {
		l.manager.UnregisterPlugin(plug.Name())
		return fmt.Errorf("init plugin: %w", err)
	}
	if err := safeCall(func() error { return plug.Start(ctx) }); err != nil {
		l.manager.UnregisterPlugin(plug.Name())
		return fmt.Errorf("start plugin: %w", err)
	}

	l.mu.Lock()
	l.loaded[name] = &LoadedPlugin{
		Plugin:   plug,
		Handle:   p,
		Path:     path,
		Metadata: meta,
		LoadedAt: time.Now(),
	}
	if digest, err := fileDigest(path); err == nil {
		l.digests[name] = digest
	}
	l.mu.Unlock()
	l.logger.Info("plugin loaded", "name", name, "path", path)
	return nil
}

// fileSwapped reports whether the bytes at <dir>/<name>.so differ from what
// this process opened before (loaded or failed to load). reload of an
// unchanged file is allowed; different bytes need a restart.
func (l *Loader) fileSwapped(name, path string) (bool, string) {
	l.mu.RLock()
	known := l.digests[name]
	l.mu.RUnlock()
	if known == "" {
		return false, ""
	}
	digest, err := fileDigest(path)
	if err != nil {
		return false, ""
	}
	return digest != known, digest
}

// fileDigest is the sha256 of the file's bytes, "" when unreadable.
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// safeCall keeps a panicking plugin from taking the process down.
func safeCall(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}

// LoadByName loads a plugin by name from the plugins directory.
func (l *Loader) LoadByName(ctx context.Context, name string) error {
	name = strings.TrimSuffix(name, ".so")
	if !ValidName(name) {
		return ErrBadName
	}
	l.op.Lock()
	defer l.op.Unlock()
	return l.load(ctx, filepath.Join(l.dir, name+".so"))
}

// Unload unloads a plugin by name.
func (l *Loader) Unload(ctx context.Context, name string) error {
	l.op.Lock()
	defer l.op.Unlock()
	return l.unload(ctx, name)
}

func (l *Loader) unload(ctx context.Context, name string) error {
	l.mu.RLock()
	loaded, ok := l.loaded[name]
	l.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s: %w", name, ErrNotInstalled)
	}
	if err := safeCall(func() error { return loaded.Plugin.Stop(ctx) }); err != nil {
		l.logger.Warn("plugin stop error", "name", name, "error", err)
	}
	l.manager.UnregisterPlugin(loaded.Plugin.Name())
	l.mu.Lock()
	delete(l.loaded, name)
	l.mu.Unlock()
	l.logger.Info("plugin unloaded", "name", name)
	return nil
}

// Reload unloads and loads one plugin again, re-reading its settings.
func (l *Loader) Reload(ctx context.Context, name string) error {
	if !ValidName(name) {
		return ErrBadName
	}
	l.op.Lock()
	defer l.op.Unlock()
	if err := l.unload(ctx, name); err != nil {
		return err
	}
	return l.load(ctx, filepath.Join(l.dir, name+".so"))
}

// Install downloads a plugin .so from the registry.
func (l *Loader) Install(ctx context.Context, name string) error {
	name = strings.TrimSuffix(name, ".so")
	if !ValidName(name) {
		return ErrBadName
	}
	l.op.Lock()
	defer l.op.Unlock()
	return l.download(ctx, name)
}

func (l *Loader) download(ctx context.Context, name string) error {
	dest := filepath.Join(l.dir, name+".so")
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin %s at %s: %w", name, dest, ErrAlreadyInstalled)
	}

	url := fmt.Sprintf("%s/%s.so", strings.TrimSuffix(l.RepoURL(), "/"), name)
	l.logger.Info("downloading plugin", "name", name, "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := l.http.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("plugin %s: %w", name, ErrNotInRepo)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}

	// Write to a temp file first so a cut download never leaves a
	// truncated .so that would fail on every start.
	tmp, err := os.CreateTemp(l.dir, "."+name+"-*.so.part")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	written, err := io.Copy(tmp, resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write plugin: %w", err)
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write plugin: %w", err)
	}
	l.logger.Info("plugin downloaded", "name", name, "size", written, "path", dest)
	return nil
}

// InstallAndLoad downloads (when missing) and loads a plugin. Installed
// always means loaded: a file that fails to load is deleted again, except
// a file that was swapped for new bytes at an already-open path — those are
// kept and load on the next start.
func (l *Loader) InstallAndLoad(ctx context.Context, name string) error {
	name = strings.TrimSuffix(name, ".so")
	if !ValidName(name) {
		return ErrBadName
	}
	l.op.Lock()
	defer l.op.Unlock()
	if l.IsLoaded(name) {
		return fmt.Errorf("plugin %s: %w", name, ErrAlreadyInstalled)
	}
	// A file that is there but not loaded is broken or stale: fetch it again.
	_ = os.Remove(filepath.Join(l.dir, name+".so"))
	if err := l.download(ctx, name); err != nil {
		return err
	}
	if err := l.load(ctx, filepath.Join(l.dir, name+".so")); err != nil {
		if errors.Is(err, ErrNeedsRestart) {
			return err
		}
		_ = os.Remove(filepath.Join(l.dir, name+".so"))
		l.mu.Lock()
		delete(l.failed, name)
		l.mu.Unlock()
		return err
	}
	return nil
}

// InstallFileAndLoad installs a plugin from a local .so file (downloaded
// from a replied message), then loads it. The source file is not modified.
// The file's name is ignored: the plugin is installed under the module
// name recorded in the .so itself, so file name and plugin name can never
// drift apart.
func (l *Loader) InstallFileAndLoad(ctx context.Context, src string) error {
	l.op.Lock()
	defer l.op.Unlock()

	name, digest, err := inspectFile(src)
	if err != nil {
		return err
	}
	if l.IsLoaded(name) {
		// Already running. If the bytes are the same, that is a plain
		// "already installed"; different bytes stage for the next start.
		if l.knownDigest(name) != digest {
			if err := l.stageFrom(src, name); err != nil {
				return err
			}
			return fmt.Errorf("plugin %s: %w", name, ErrNeedsRestart)
		}
		return fmt.Errorf("plugin %s: %w", name, ErrAlreadyInstalled)
	}
	dest := filepath.Join(l.dir, name+".so")
	if _, err := os.Stat(dest); err == nil {
		if known := l.knownDigest(name); known != "" && known != digest {
			// New bytes for a file this process already opened: stage
			// them, they load on the next start.
			if err := l.stageFrom(src, name); err != nil {
				return err
			}
			return fmt.Errorf("plugin %s: %w", name, ErrNeedsRestart)
		}
		// Same bytes as the open (or failed) module: nothing to do.
		if l.IsLoaded(name) {
			return fmt.Errorf("plugin %s: %w", name, ErrAlreadyInstalled)
		}
	}
	if err := l.stageFrom(src, name); err != nil {
		return err
	}
	if err := l.load(ctx, dest); err != nil {
		if errors.Is(err, ErrNeedsRestart) {
			return err
		}
		_ = os.Remove(dest)
		l.mu.Lock()
		delete(l.failed, name)
		l.mu.Unlock()
		return err
	}
	return nil
}

// knownDigest returns the digest recorded for a name this process opened.
func (l *Loader) knownDigest(name string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.digests[name]
}

// inspectFile reads a .so well enough to know its plugin name: the ELF
// magic, and the module path baked in at build time.
func inspectFile(path string) (name, digest string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil || !bytes.Equal(magic, []byte("\x7fELF")) {
		return "", "", fmt.Errorf("%s: %w", filepath.Base(path), ErrBadFile)
	}
	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", filepath.Base(path), ErrBadFile)
	}
	mod := bi.Main.Path
	last := mod
	if i := strings.LastIndex(mod, "/"); i >= 0 {
		last = mod[i+1:]
	}
	if !ValidName(last) {
		return "", "", fmt.Errorf("%s: %w", filepath.Base(path), ErrBadFile)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", "", err
	}
	return last, fmt.Sprintf("%x", h.Sum(nil)), nil
}

// stageFrom copies src into plugins/<name>.so via a temp file, atomically.
func (l *Loader) stageFrom(src, name string) error {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	tmp, err := os.CreateTemp(l.dir, "."+name+"-*.so.part")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	written, err := io.Copy(tmp, in)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write plugin: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(l.dir, name+".so")); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write plugin: %w", err)
	}
	l.logger.Info("plugin file staged", "name", name, "size", written)
	return nil
}

// Remove unloads a plugin and deletes its .so file. Its settings stay in
// data/<name>/ so a reinstall picks them up again.
func (l *Loader) Remove(ctx context.Context, name string) error {
	name = strings.TrimSuffix(name, ".so")
	if !ValidName(name) {
		return ErrBadName
	}
	l.op.Lock()
	defer l.op.Unlock()

	if l.IsLoaded(name) {
		if err := l.unload(ctx, name); err != nil {
			l.logger.Warn("unload before remove", "name", name, "error", err)
		}
	}

	path := filepath.Join(l.dir, name+".so")
	if err := os.Remove(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin %s: %w", name, ErrNotInstalled)
	} else if err != nil {
		return fmt.Errorf("remove plugin file: %w", err)
	}
	l.mu.Lock()
	delete(l.failed, name)
	delete(l.digests, name)
	l.mu.Unlock()
	l.logger.Info("plugin removed", "name", name)
	return nil
}

// GetLoaded returns all currently loaded plugins.
func (l *Loader) GetLoaded() map[string]*LoadedPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make(map[string]*LoadedPlugin, len(l.loaded))
	for k, v := range l.loaded {
		result[k] = v
	}
	return result
}

// LoadedNames returns the loaded plugin names, sorted.
func (l *Loader) LoadedNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, len(l.loaded))
	for n := range l.loaded {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// IsLoaded checks if a plugin is loaded.
func (l *Loader) IsLoaded(name string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.loaded[name]
	return ok
}

// Failed returns plugin files in the directory that did not load, with
// the reason.
func (l *Loader) Failed() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make(map[string]string, len(l.failed))
	for k, v := range l.failed {
		if _, err := os.Stat(filepath.Join(l.dir, k+".so")); err == nil {
			out[k] = v
		}
	}
	return out
}

// GetAvailable returns .so files in the plugins dir that are NOT loaded.
func (l *Loader) GetAvailable() ([]string, error) {
	installed, err := l.GetInstalled()
	if err != nil {
		return nil, err
	}
	var available []string
	for _, name := range installed {
		if !l.IsLoaded(name) {
			available = append(available, name)
		}
	}
	return available, nil
}

// GetInstalled returns all .so files in the plugins dir.
func (l *Loader) GetInstalled() ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var installed []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".so") {
			continue
		}
		installed = append(installed, strings.TrimSuffix(e.Name(), ".so"))
	}
	return installed, nil
}

// GetPluginDir returns the plugin directory path.
func (l *Loader) GetPluginDir() string {
	return l.dir
}

// RepoURL returns the configured plugin registry base URL.
func (l *Loader) RepoURL() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.repoURL
}

// SetRepoURL sets the plugin registry URL.
func (l *Loader) SetRepoURL(url string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if url != l.repoURL {
		l.index, l.indexAt = nil, time.Time{}
	}
	l.repoURL = url
}

// indexTTL keeps paging through the repository from refetching it.
const indexTTL = time.Minute

// Index fetches the repository index, sorted by name. A fetch within the
// last minute is reused unless fresh is set.
func (l *Loader) Index(ctx context.Context, fresh bool) ([]IndexEntry, error) {
	l.mu.RLock()
	cached, at := l.index, l.indexAt
	l.mu.RUnlock()
	if !fresh && cached != nil && time.Since(at) < indexTTL {
		return append([]IndexEntry(nil), cached...), nil
	}
	url := strings.TrimSuffix(l.RepoURL(), "/") + "/plugins.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var entries []IndexEntry
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&entries); err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if ValidName(e.Name) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	l.mu.Lock()
	l.index, l.indexAt = out, time.Now()
	l.mu.Unlock()
	return append([]IndexEntry(nil), out...), nil
}

// Explain turns loader errors into a short reason in both languages.
func Explain(err error) (zh, en string) {
	msg := err.Error()
	var mismatch *NameMismatchError
	switch {
	case errors.Is(err, ErrAlreadyInstalled):
		return "已经装过了", "already installed"
	case errors.Is(err, ErrNotInstalled):
		return "没装这个插件", "not installed"
	case errors.Is(err, ErrNotInRepo):
		return "仓库里没有这个插件", "not in the repository"
	case errors.Is(err, ErrBadName):
		return "插件名不合法", "invalid plugin name"
	case errors.Is(err, ErrBadFile):
		return "不是能用的插件文件（要 Linux 的 .so）", "not a usable plugin file (a Linux .so is required)"
	case errors.Is(err, ErrNeedsRestart):
		return "这个进程已经加载过它，新文件已放好，`restart` 后生效",
			"already opened in this process; the new file is staged and loads after `restart`"
	case errors.As(err, &mismatch):
		return fmt.Sprintf("文件里的插件叫 %s，文件得叫 %s.so", mismatch.Plugin, mismatch.Plugin),
			fmt.Sprintf("the plugin inside is %s; the file must be named %s.so", mismatch.Plugin, mismatch.Plugin)
	case strings.Contains(msg, "different version of package"):
		return "和主程序的 Go 版本不一致，先 `update -f` 重装主程序再装",
			"built with a different Go version; run `update -f` first"
	case strings.Contains(msg, "plugin already loaded"):
		return "本进程已加载过这个插件的别的文件，`restart` 后再装",
			"another copy of this plugin was already loaded; `restart` first"
	case strings.Contains(msg, "already registered"):
		return "和已有插件或命令重名", "clashes with an existing plugin or command"
	}
	return msg, msg
}
