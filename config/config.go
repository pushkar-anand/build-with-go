// Package config loads application configuration from a YAML file, with
// environment variables layered on top for values that should not live in the
// file — secrets, and anything that differs per deployment.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	delim = "."

	// defaultTagName is the struct tag Load reads field names from.
	defaultTagName = "koanf"

	// nestSeparator marks a level of nesting in an environment variable name.
	//
	// A single underscore stays part of the key, so a key whose name contains
	// one is still reachable from the environment:
	//
	//	SERVER__READ_TIMEOUT  ->  server.read_timeout
	nestSeparator = "__"
)

// k is a process-wide store. Successive loads merge into it, which is koanf's
// model: each Load layers over whatever is already there rather than replacing
// it. Loading twice in one process is therefore cumulative, not independent.
var k = koanf.New(delim)

type (
	// Option configures Load.
	Option interface {
		apply(*loader)
	}

	optionFunc func(*loader)

	loader struct {
		yamlFile  string
		envPrefix string
		defaults  map[string]any
		tagName   string
		environ   func() []string

		// splitSep is the separator WithSplitKeyFor gave, and splitKeys the
		// key patterns whose environment value is split at it.
		splitSep  string
		splitKeys map[string]struct{}
	}
)

func (fn optionFunc) apply(l *loader) {
	fn(l)
}

// WithYAML reads configuration from a YAML file.
//
// A missing file is not an error, so the same code path works for a deployment
// that supplies everything through the environment.
func WithYAML(path string) Option {
	return optionFunc(func(l *loader) {
		l.yamlFile = path
	})
}

// WithDefaults seeds values that any later source may override. Keys are
// dot-separated paths, for example "server.read_timeout".
func WithDefaults(values map[string]any) Option {
	return optionFunc(func(l *loader) {
		l.defaults = values
	})
}

// WithEnvPrefix restricts the environment to variables carrying the prefix, and
// strips it from the resulting key. The prefix is case-sensitive: "APP_".
func WithEnvPrefix(prefix string) Option {
	return optionFunc(func(l *loader) {
		l.envPrefix = prefix
	})
}

// WithTag overrides the struct tag field names are read from. Defaults to "koanf".
func WithTag(tag string) Option {
	return optionFunc(func(l *loader) {
		l.tagName = tag
	})
}

// WithEnviron replaces the source of environment variables, which lets a test
// supply its own set instead of mutating the process environment.
func WithEnviron(fn func() []string) Option {
	return optionFunc(func(l *loader) {
		l.environ = fn
	})
}

// WithSplitKeyFor names keys whose environment variable holds a list, its items
// separated by sep, since a variable holds one string:
//
//	WithSplitKeyFor(",", "server.allowed_origins")
//	APP_SERVER__ALLOWED_ORIGINS=https://a.example,https://b.example
//
// A value is split only when it contains sep. Each item is trimmed of space
// and empty items are dropped, so a sep of " " takes any run of spaces. A
// value without sep stays one string, which decodes into a one-item list. A
// "*" in a key matches any one segment, for a list inside a map:
// "plugins.*.hosts". Every other key keeps its value whole, separators and
// all, so name only list keys. Keys from several calls add up, and the sep
// last given applies to all of them. An empty sep is ignored.
func WithSplitKeyFor(sep string, keys ...string) Option {
	return optionFunc(func(l *loader) {
		if sep == "" {
			return
		}

		if l.splitKeys == nil {
			l.splitKeys = make(map[string]struct{}, len(keys))
		}

		l.splitSep = sep

		for _, key := range keys {
			l.splitKeys[key] = struct{}{}
		}
	})
}

// Load reads configuration into a T.
//
// Sources are layered, each overriding the one before it:
//
//  1. defaults given to WithDefaults
//  2. the YAML file given to WithYAML, if it exists
//  3. environment variables
//
// Environment variable names map onto config keys by stripping the prefix,
// lowercasing, and treating a double underscore as a nesting separator:
//
//	APP_DATABASE__PASSWORD  ->  database.password
//	APP_SERVER__READ_TIMEOUT -> server.read_timeout
func Load[T any](opts ...Option) (*T, error) {
	l := &loader{tagName: defaultTagName}

	for _, opt := range opts {
		opt.apply(l)
	}

	if len(l.defaults) > 0 {
		err := k.Load(confmap.Provider(l.defaults, delim), nil)
		if err != nil {
			return nil, fmt.Errorf("config: loading defaults: %w", err)
		}
	}

	if l.yamlFile != "" {
		err := k.Load(file.Provider(l.yamlFile), yaml.Parser())
		if err != nil && !fileNotExistsErr(err) {
			return nil, fmt.Errorf("config: loading %s: %w", l.yamlFile, err)
		}
	}

	err := k.Load(env.Provider(delim, l.envOpt()), nil)
	if err != nil {
		return nil, fmt.Errorf("config: loading environment: %w", err)
	}

	return unmarshal[T](l.tagName)
}

func (l *loader) envOpt() env.Opt {
	return env.Opt{
		Prefix:      l.envPrefix,
		EnvironFunc: l.environ,
		TransformFunc: func(key, value string) (string, any) {
			path := envKeyToPath(l.envPrefix, key)

			if !strings.Contains(value, l.splitSep) || !l.splits(path) {
				return path, value
			}

			return path, splitItems(value, l.splitSep)
		},
	}
}

// envKeyToPath maps an environment variable name onto a config key path.
func envKeyToPath(prefix, key string) string {
	trimmed := strings.ToLower(strings.TrimPrefix(key, prefix))

	return strings.ReplaceAll(trimmed, nestSeparator, delim)
}

// splits reports whether WithSplitKeyFor named path, a "*" in a key matching
// any one segment.
func (l *loader) splits(path string) bool {
	if _, ok := l.splitKeys[path]; ok {
		return true
	}

	segments := strings.Split(path, delim)

	for pattern := range l.splitKeys {
		if strings.Contains(pattern, "*") && matchKey(strings.Split(pattern, delim), segments) {
			return true
		}
	}

	return false
}

// matchKey reports whether path matches pattern segment by segment, a "*"
// segment matching any one.
func matchKey(pattern, path []string) bool {
	if len(pattern) != len(path) {
		return false
	}

	for i, p := range pattern {
		if p != "*" && p != path[i] {
			return false
		}
	}

	return true
}

// splitItems splits value at sep, trimming each item and dropping empty ones.
func splitItems(value, sep string) []string {
	var items []string

	for item := range strings.SplitSeq(value, sep) {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}

	return items
}

func unmarshal[T any](tag string) (*T, error) {
	c := new(T)

	err := k.UnmarshalWithConf("", c, koanf.UnmarshalConf{Tag: tag})
	if err != nil {
		return nil, fmt.Errorf("config: unmarshalling into %T: %w", c, err)
	}

	return c, nil
}

func fileNotExistsErr(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist)
}
