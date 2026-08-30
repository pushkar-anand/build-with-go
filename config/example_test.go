package config_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pushkar-anand/build-with-go/config"
)

type (
	Server struct {
		Host        string        `koanf:"host"`
		Port        int           `koanf:"port"`
		ReadTimeout time.Duration `koanf:"read_timeout"`
	}

	Database struct {
		DSN      string `koanf:"dsn"`
		Password string `koanf:"password"`
	}

	Config struct {
		Server   Server   `koanf:"server"`
		Database Database `koanf:"database"`
	}
)

// Each source overrides the one before it, so the file carries what is safe to
// commit and the environment carries the rest.
func Example() {
	dir, err := os.MkdirTemp("", "config")
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "config.yaml")

	err = os.WriteFile(path, []byte("server:\n  port: 9000\ndatabase:\n  dsn: postgres://localhost/app\n"), 0o600)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load[Config](
		config.WithDefaults(map[string]any{
			"server.host":         "0.0.0.0",
			"server.port":         8080,
			"server.read_timeout": "15s",
		}),
		config.WithYAML(path),
		config.WithEnvPrefix("APP_"),
		// Normally the real environment. A double underscore separates nesting,
		// so read_timeout keeps the underscore in its own name.
		config.WithEnviron(func() []string {
			return []string{
				"APP_SERVER__READ_TIMEOUT=60s",
				"APP_DATABASE__PASSWORD=hunter2",
			}
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("host:    ", cfg.Server.Host, "(default)")
	fmt.Println("port:    ", cfg.Server.Port, "(yaml over default)")
	fmt.Println("timeout: ", cfg.Server.ReadTimeout, "(env over yaml)")
	fmt.Println("password:", cfg.Database.Password, "(env only)")

	// Output:
	// host:     0.0.0.0 (default)
	// port:     9000 (yaml over default)
	// timeout:  1m0s (env over yaml)
	// password: hunter2 (env only)
}
