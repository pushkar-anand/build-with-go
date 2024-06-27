package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	envTagName = "env"
)

var (
	k = koanf.New(".") // global koanf instance
)

var envKeyModifier = func(prefix string) func(string) string {
	return func(s string) string {
		return strings.Replace(
			strings.ToLower(
				strings.TrimPrefix(s, prefix),
			), "_", ".", -1,
		)
	}
}

const delim = "."

func ReadFromEnv[T any](envFile string, prefix string) (*T, error) {
	envFileParser := dotenv.ParserEnv(prefix, delim, envKeyModifier(prefix))
	osEnvProvider := env.Provider(prefix, delim, envKeyModifier(prefix))

	err := k.Load(file.Provider(envFile), envFileParser)
	if err != nil && !fileNotExistsErr(err) {
		return nil, fmt.Errorf("error loading config from %s: %w", envFile, err)
	}

	err = k.Load(osEnvProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("error loading config from %s: %w", envFile, err)
	}

	c, err := unmarshalConfig[T](k)
	if err != nil {
		return nil, fmt.Errorf("failed: %w", err)
	}

	return c, nil
}

func unmarshalConfig[T any](k *koanf.Koanf) (*T, error) {
	c := new(T)

	// Unmarshal the whole thing into a struct.
	err := k.UnmarshalWithConf("", c, koanf.UnmarshalConf{
		Tag: envTagName,
	})
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	return c, nil
}

func fileNotExistsErr(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist)
}

func easyPrint(data interface{}) {
	manifestJson, _ := json.MarshalIndent(data, "", "  ")

	log.Println(string(manifestJson))
}
