package loader

import (
	"fmt"
	"reflect"

	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// instantiate calls a plugin's exported New symbol. Any zero-argument
// function is accepted as long as its first result implements
// pkgplugin.Plugin and an optional second result is an error, e.g.
// func() *MyPlugin, func() (pkgplugin.Plugin, error), func() interface{}.
func instantiate(sym interface{}) (pkgplugin.Plugin, error) {
	v := reflect.ValueOf(sym)
	if !v.IsValid() || v.Kind() != reflect.Func || v.IsNil() {
		return nil, fmt.Errorf("New symbol is not a function (got %T)", sym)
	}
	t := v.Type()
	if t.NumIn() != 0 || t.NumOut() < 1 || t.NumOut() > 2 ||
		(t.NumOut() == 2 && !t.Out(1).Implements(errorType)) {
		return nil, fmt.Errorf("New has unsupported signature %s", t)
	}

	out := v.Call(nil)
	if len(out) == 2 && !out[1].IsNil() {
		return nil, fmt.Errorf("plugin New failed: %w", out[1].Interface().(error))
	}
	first := out[0]
	switch first.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if first.IsNil() {
			return nil, fmt.Errorf("plugin New returned nil")
		}
	}
	plug, ok := first.Interface().(pkgplugin.Plugin)
	if !ok {
		return nil, fmt.Errorf("plugin does not implement plugin.Plugin interface (New returns %s)", t.Out(0))
	}
	return plug, nil
}
