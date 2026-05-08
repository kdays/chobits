package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type DotEnvLoadResult struct {
	Loaded []string
	Errors []error
}

func (result DotEnvLoadResult) Err() error {
	if len(result.Errors) == 0 {
		return nil
	}
	return errors.Join(result.Errors...)
}

func LoadDotEnv(paths ...string) error {
	result := TryLoadDotEnv(paths...)
	if len(result.Loaded) > 0 {
		return nil
	}
	return result.Err()
}

func TryLoadDotEnv(paths ...string) DotEnvLoadResult {
	result := DotEnvLoadResult{}
	for _, path := range paths {
		if path == "" {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}

		entries, err := parseDotEnv(file, path)
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}

		for _, entry := range entries {
			if _, exists := os.LookupEnv(entry.key); exists {
				continue
			}
			if err := os.Setenv(entry.key, entry.value); err != nil {
				result.Errors = append(result.Errors, err)
				break
			}
		}
		result.Loaded = append(result.Loaded, path)
	}
	return result
}

type dotEnvEntry struct {
	key   string
	value string
}

func parseDotEnv(file *os.File, name string) ([]dotEnvEntry, error) {
	entries := []dotEnvEntry{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: missing '='", name, lineNo)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", name, lineNo)
		}

		parsed, err := parseDotEnvValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, lineNo, err)
		}
		entries = append(entries, dotEnvEntry{key: key, value: parsed})
	}
	return entries, scanner.Err()
}

func parseDotEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, `"`) {
		return strconv.Unquote(value)
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'"), nil
	}
	if beforeComment, _, found := strings.Cut(value, " #"); found {
		value = beforeComment
	}
	return strings.TrimSpace(value), nil
}
