package config

import (
	"errors"
	"fmt"
	"io/fs"
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
			strings.ToUpper(
				strings.TrimPrefix(s, prefix),
			), "_", ".", -1,
		)
	}
}

func ReadFromEnv[T any](envFile string, prefix string) (*T, error) {
	err := k.Load(file.Provider(envFile), dotenv.ParserEnv(prefix, ".", envKeyModifier(prefix)))
	if err != nil && !fileNotExistsErr(err) {
		return nil, fmt.Errorf("error loading config from %s: %w", envFile, err)
	}

	err = k.Load(env.Provider(prefix, ".", envKeyModifier(prefix)), nil)
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
