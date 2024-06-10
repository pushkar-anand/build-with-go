package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

type (
	Server struct {
		Host string `env:"SERVER.HOST"`
		Port int    `env:"SERVER.PORT"`
	}

	Config struct {
		Server Server
	}
)

func TestReadFromEnv(t *testing.T) {
	t.Run("read from OS env", func(t *testing.T) {
		err := os.Setenv("SERVER_HOST", "0.0.0.0")
		assert.NoError(t, err)

		err = os.Setenv("SERVER_PORT", "8080")
		assert.NoError(t, err)

		cfg, err := ReadFromEnv[Config](".env", "")
		assert.NoError(t, err)

		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
		assert.Equal(t, 8080, cfg.Server.Port)
	})

	t.Run("read from env file", func(t *testing.T) {
		fileName := ".env"

		err := createEnvFileForTest(t, fileName)
		require.NoError(t, err)

		cfg, err := ReadFromEnv[Config](fileName, "")
		assert.NoError(t, err)

		assert.Equal(t, "localhost", cfg.Server.Host)
		assert.Equal(t, 8080, cfg.Server.Port)
	})

	t.Run("read from .env file with override from OS", func(t *testing.T) {
		fileName := ".test.env"

		err := createEnvFileForTest(t, fileName)
		require.NoError(t, err)

		err = os.Setenv("SERVER_HOST", "0.0.0.0")
		assert.NoError(t, err)

		cfg, err := ReadFromEnv[Config](fileName, "")
		assert.NoError(t, err)

		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
		assert.Equal(t, 8080, cfg.Server.Port)
	})
}

func createEnvFileForTest(t *testing.T, fileName string) error {
	t.Helper()

	err := os.WriteFile(fileName, []byte("SERVER_HOST=localhost\nSERVER_PORT=8080\n"), 0644)
	if err != nil {
		return err
	}

	//t.Cleanup(func() {
	//	_ = os.Remove(fileName)
	//})

	return nil
}
