package convert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func String(value any) string {
	if value == nil {
		return ""
	}
	reflectValue := reflect.ValueOf(value)
	switch reflectValue.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflectValue.IsNil() {
			return ""
		}
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case json.Number:
		return typed.String()
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return typed.String()
	case error:
		return typed.Error()
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		switch reflectValue.Kind() {
		case reflect.String:
			return reflectValue.String()
		case reflect.Bool:
			return strconv.FormatBool(reflectValue.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.FormatInt(reflectValue.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return strconv.FormatUint(reflectValue.Uint(), 10)
		case reflect.Float32:
			return strconv.FormatFloat(reflectValue.Float(), 'f', -1, 32)
		case reflect.Float64:
			return strconv.FormatFloat(reflectValue.Float(), 'f', -1, 64)
		case reflect.Slice:
			if reflectValue.Type().Elem().Kind() == reflect.Uint8 {
				return string(reflectValue.Bytes())
			}
		}
		data, err := json.Marshal(typed)
		if err == nil {
			return string(data)
		}
		return fmt.Sprint(typed)
	}
}

func StringOr(value any, fallback string) string {
	converted := String(value)
	if converted == "" {
		return fallback
	}
	return converted
}

func Int(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}

func IntOr(value string, fallback int) int {
	parsed, err := Int(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Int64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

func Int64Or(value string, fallback int64) int64 {
	parsed, err := Int64(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Uint(value string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 0)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

func UintOr(value string, fallback uint) uint {
	parsed, err := Uint(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Uint64(value string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(value), 10, 64)
}

func Uint64Or(value string, fallback uint64) uint64 {
	parsed, err := Uint64(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Float64(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func Float64Or(value string, fallback float64) float64 {
	parsed, err := Float64(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Bool(value string) (bool, error) {
	return strconv.ParseBool(strings.TrimSpace(value))
}

func BoolOr(value string, fallback bool) bool {
	parsed, err := Bool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
