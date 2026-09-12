// Package i18n provides native multi-language support for PaperValet.
// Every user-facing string should go through the catalog.
package i18n

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Lang is a language code like "zh-CN" or "en-US".
type Lang string

const (
	ZhCN Lang = "zh-CN"
	EnUS Lang = "en-US"
)

// Catalog is a translation table: lang -> key -> template.
// Templates use {0}, {1} style placeholders.
type Catalog struct {
	mu       sync.RWMutex
	langs    map[Lang]map[string]string
	default_ Lang
}

// New creates an empty catalog with the given default language.
func New(defaultLang Lang) *Catalog {
	return &Catalog{
		langs:    make(map[Lang]map[string]string),
		default_: defaultLang,
	}
}

// SetDefault changes the default language.
func (c *Catalog) SetDefault(l Lang) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.default_ = l
}

// Default returns the default language.
func (c *Catalog) Default() Lang {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.default_
}

// Add registers a translation for a language.
func (c *Catalog) Add(l Lang, key, template string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.langs[l] == nil {
		c.langs[l] = make(map[string]string)
	}
	c.langs[l][key] = template
}

// AddMany registers multiple translations for a language.
func (c *Catalog) AddMany(l Lang, entries map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.langs[l] == nil {
		c.langs[l] = make(map[string]string)
	}
	for k, v := range entries {
		c.langs[l][k] = v
	}
}

// Merge merges another catalog's entries into this one.
func (c *Catalog) Merge(other *Catalog) {
	other.mu.RLock()
	defer other.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	for l, entries := range other.langs {
		if c.langs[l] == nil {
			c.langs[l] = make(map[string]string)
		}
		for k, v := range entries {
			c.langs[l][k] = v
		}
	}
}

// T translates a key into the given language, substituting {N} placeholders.
// Fallback chain: requested lang → default lang → key itself.
func (c *Catalog) T(l Lang, key string, args ...any) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tmpl, ok := c.langs[l][key]
	if !ok {
		tmpl, ok = c.langs[c.default_][key]
	}
	if !ok {
		if len(args) == 0 {
			return key
		}
		// Avoid printf-wrapper detection: build the fallback manually.
		var b strings.Builder
		b.WriteString(key)
		for _, a := range args {
			b.WriteString(" ")
			b.WriteString(fmt.Sprint(a))
		}
		return b.String()
	}
	if len(args) == 0 {
		return tmpl
	}
	return substitute(tmpl, args)
}

// Has reports whether a key exists in the given language (or default).
func (c *Catalog) Has(l Lang, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.langs[l][key]; ok {
		return true
	}
	_, ok := c.langs[c.default_][key]
	return ok
}

// Languages returns all registered languages, sorted.
func (c *Catalog) Languages() []Lang {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Lang, 0, len(c.langs))
	for l := range c.langs {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Keys returns all keys registered for a language, sorted.
func (c *Catalog) Keys(l Lang) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entries := c.langs[l]
	out := make([]string, 0, len(entries))
	for k := range entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// substitute replaces {N} placeholders with args.
func substitute(tmpl string, args []any) string {
	var b strings.Builder
	for i := 0; i < len(tmpl); i++ {
		ch := tmpl[i]
		if ch == '{' {
			end := strings.IndexByte(tmpl[i:], '}')
			if end > 0 {
				idx := 0
				if _, err := fmt.Sscanf(tmpl[i+1:i+end], "%d", &idx); err == nil && idx >= 0 && idx < len(args) {
					b.WriteString(fmt.Sprint(args[idx]))
					i += end
					continue
				}
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}