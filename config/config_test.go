package config

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	Server struct {
		Host string `env:"host"`
		Port int    `env:"port"`
	}

	Config struct {
		Server Server `env:"server"`
	}
)

func TestReadFromEnv_readFromOSEnv(t *testing.T) {
	host := "0.0.0.0"
	port := 8080

	t.Setenv("SERVER_HOST", host)
	t.Setenv("SERVER_PORT", strconv.Itoa(port))

	cfg, err := ReadFromEnv[Config](".env", "")
	require.NoError(t, err)

	assert.Equal(t, host, cfg.Server.Host)
	assert.Equal(t, port, cfg.Server.Port)
}

func TestReadFromEnv_readFromDotEnvFile(t *testing.T) {
	host := "localhost"
	port := 8080
	fileName := ".test.env"
	vars := fmt.Sprintf("SERVER_HOST=%s\nSERVER_PORT=%d", host, port)

	err := createEnvFileForTest(t, fileName, vars)
	require.NoError(t, err)

	cfg, err := ReadFromEnv[Config](fileName, "")
	require.NoError(t, err)

	assert.Equal(t, host, cfg.Server.Host)
	assert.Equal(t, port, cfg.Server.Port)
}

func TestReadFromEnv_readFromFileOverrideOS(t *testing.T) {
	host := "0.0.0.0"
	port := 8080
	fileName := ".test.env"

	err := createEnvFileForTest(t, fileName, "SERVER_HOST=localhost\n")
	require.NoError(t, err)

	t.Setenv("SERVER_HOST", host)
	t.Setenv("SERVER_PORT", strconv.Itoa(port))

	cfg, err := ReadFromEnv[Config](fileName, "")
	require.NoError(t, err)

	assert.Equal(t, host, cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func createEnvFileForTest(t *testing.T, fileName string, data string) error {
	t.Helper()

	err := os.WriteFile(fileName, []byte(data), 0644)
	if err != nil {
		return err
	}

	t.Cleanup(func() {
		_ = os.Remove(fileName)
	})

	return nil
}
