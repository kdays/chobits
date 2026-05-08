package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func ApplyEnv(target any, prefix string) error {
	if target == nil {
		return nil
	}
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("env target must be a non-nil pointer")
	}
	return applyEnvValue(value.Elem(), envPrefixParts(prefix), nil)
}

func applyEnvValue(value reflect.Value, prefix []string, path []string) error {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return applyEnvValue(value.Elem(), prefix, path)
	}

	switch value.Kind() {
	case reflect.Struct:
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name, inline, skip := yamlFieldName(field)
			if skip {
				continue
			}
			nextPath := path
			if !inline {
				nextPath = appendPath(path, name)
			}
			if err := applyEnvValue(value.Field(i), prefix, nextPath); err != nil {
				return err
			}
		}
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return nil
		}
		for _, key := range value.MapKeys() {
			item := value.MapIndex(key)
			if !item.IsValid() {
				continue
			}
			copyValue := reflect.New(item.Type()).Elem()
			copyValue.Set(item)
			if err := applyEnvValue(copyValue, prefix, appendPath(path, key.String())); err != nil {
				return err
			}
			value.SetMapIndex(key, copyValue)
		}
	default:
		envName := envName(prefix, path)
		if envName == "" {
			return nil
		}
		raw, ok := os.LookupEnv(envName)
		if !ok {
			return nil
		}
		if err := setValueFromString(value, raw); err != nil {
			return fmt.Errorf("%s: %w", envName, err)
		}
	}
	return nil
}

func yamlFieldName(field reflect.StructField) (name string, inline bool, skip bool) {
	tag := field.Tag.Get("yaml")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	if len(parts) > 1 {
		for _, option := range parts[1:] {
			if option == "inline" {
				inline = true
				break
			}
		}
	}
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	return name, inline, false
}

func appendPath(path []string, value string) []string {
	next := make([]string, 0, len(path)+1)
	next = append(next, path...)
	next = append(next, value)
	return next
}

func envPrefixParts(prefix string) []string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}
	return []string{prefix}
}

func envName(prefix []string, path []string) string {
	if len(path) == 0 {
		return ""
	}
	parts := make([]string, 0, len(prefix)+len(path))
	parts = append(parts, prefix...)
	parts = append(parts, path...)
	for i, part := range parts {
		part = strings.ReplaceAll(part, "-", "_")
		part = strings.ReplaceAll(part, ".", "_")
		parts[i] = strings.ToUpper(part)
	}
	return strings.Join(parts, "_")
}

func setValueFromString(value reflect.Value, raw string) error {
	if !value.CanSet() {
		return nil
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return setValueFromString(value.Elem(), raw)
	}

	switch value.Kind() {
	case reflect.String:
		value.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		value.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(raw, 10, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(raw, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetFloat(parsed)
	case reflect.Slice:
		if value.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice type %s", value.Type().String())
		}
		if strings.TrimSpace(raw) == "" {
			value.Set(reflect.MakeSlice(value.Type(), 0, 0))
			return nil
		}
		parts := strings.Split(raw, ",")
		result := reflect.MakeSlice(value.Type(), 0, len(parts))
		for _, part := range parts {
			result = reflect.Append(result, reflect.ValueOf(strings.TrimSpace(part)))
		}
		value.Set(result)
	default:
		return fmt.Errorf("unsupported env override type %s", value.Type().String())
	}
	return nil
}
