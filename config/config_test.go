package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Loads merge into one process-wide store, so each test owns a distinct
// top-level key rather than relying on isolation that does not exist.
type (
	server struct {
		Host        string        `koanf:"host"`
		ReadTimeout time.Duration `koanf:"read_timeout"`
	}

	yamlCfg struct {
		Section server `koanf:"fromyaml"`
	}

	envCfg struct {
		Section server `koanf:"fromenv"`
	}

	defaultsCfg struct {
		Section server `koanf:"fromdefaults"`
	}

	missingCfg struct {
		Section server `koanf:"frommissing"`
	}
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func environ(vars ...string) Option {
	return WithEnviron(func() []string { return vars })
}

func TestLoad_YAML(t *testing.T) {
	path := writeYAML(t, "fromyaml:\n  host: localhost\n  read_timeout: 15s\n")

	cfg, err := Load[yamlCfg](WithYAML(path), environ())
	require.NoError(t, err)

	assert.Equal(t, "localhost", cfg.Section.Host)
	assert.Equal(t, 15*time.Second, cfg.Section.ReadTimeout)
}

// The environment wins over the file, and a key containing an underscore is
// reachable — the old mapping turned every underscore into a nesting level, so
// read_timeout could not be set from the environment at all.
func TestLoad_EnvOverridesYAML(t *testing.T) {
	path := writeYAML(t, "fromenv:\n  host: localhost\n  read_timeout: 1s\n")

	cfg, err := Load[envCfg](
		WithYAML(path),
		WithEnvPrefix("APP_"),
		environ(
			"APP_FROMENV__HOST=0.0.0.0",
			"APP_FROMENV__READ_TIMEOUT=30s",
			"UNPREFIXED_FROMENV__HOST=ignored",
		),
	)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Section.Host)
	assert.Equal(t, 30*time.Second, cfg.Section.ReadTimeout)
}

func TestLoad_DefaultsAreOverridable(t *testing.T) {
	path := writeYAML(t, "fromdefaults:\n  host: from-yaml\n")

	cfg, err := Load[defaultsCfg](
		WithDefaults(map[string]any{
			"fromdefaults.host":         "from-defaults",
			"fromdefaults.read_timeout": "5s",
		}),
		WithYAML(path),
		environ(),
	)
	require.NoError(t, err)

	assert.Equal(t, "from-yaml", cfg.Section.Host, "yaml should beat defaults")
	assert.Equal(t, 5*time.Second, cfg.Section.ReadTimeout, "default should survive when nothing overrides it")
}

// A deployment that supplies everything through the environment should not need
// the file to exist.
func TestLoad_MissingYAMLFileIsNotAnError(t *testing.T) {
	cfg, err := Load[missingCfg](
		WithYAML(filepath.Join(t.TempDir(), "absent.yaml")),
		environ("FROMMISSING__HOST=0.0.0.0"),
	)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Section.Host)
}

func Test_envKeyToPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{name: "single word", key: "HOST", want: "host"},
		{name: "double underscore nests", key: "SERVER__HOST", want: "server.host"},
		{name: "single underscore stays in the key", key: "SERVER__READ_TIMEOUT", want: "server.read_timeout"},
		{name: "several levels", key: "A__B__C_D", want: "a.b.c_d"},
		{name: "prefix is stripped", prefix: "APP_", key: "APP_SERVER__HOST", want: "server.host"},
		{name: "prefix match is case-sensitive", prefix: "APP_", key: "app_SERVER__HOST", want: "app_server.host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, envKeyToPath(tt.prefix, tt.key))
		})
	}
}
