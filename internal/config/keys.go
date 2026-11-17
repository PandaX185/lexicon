package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Keys returns every settable dotted key in declaration order.
func Keys() []string {
	return collectKeys(reflect.TypeFor[Config](), "")
}

// Get returns the value of a dotted key as text.
func Get(cfg Config, key string) (string, error) {
	field, err := fieldByKey(reflect.ValueOf(cfg), key)
	if err != nil {
		return "", err
	}
	switch field.Kind() {
	case reflect.String:
		return field.String(), nil
	case reflect.Bool:
		return strconv.FormatBool(field.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(field.Int(), 10), nil
	default:
		return "", fmt.Errorf("key %q has unsupported type %s", key, field.Kind())
	}
}

// Set assigns a dotted key from text and validates the result.
func Set(cfg *Config, key, value string) error {
	field, err := fieldByKey(reflect.ValueOf(cfg).Elem(), key)
	if err != nil {
		return err
	}
	if !field.CanSet() {
		return fmt.Errorf("key %q is not settable", key)
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(strings.TrimSpace(value))
	case reflect.Bool:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s must be true or false: %w", key, err)
		}
		field.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return fmt.Errorf("%s must be an integer: %w", key, err)
		}
		field.SetInt(parsed)
	default:
		return fmt.Errorf("key %q has unsupported type %s", key, field.Kind())
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	return nil
}

func collectKeys(structType reflect.Type, prefix string) []string {
	var keys []string
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("toml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}
		if field.Type.Kind() == reflect.Struct {
			keys = append(keys, collectKeys(field.Type, key)...)
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func fieldByKey(value reflect.Value, key string) (reflect.Value, error) {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("key %q does not address a struct", key)
	}
	parts := strings.Split(key, ".")
	current := value
	for i, part := range parts {
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("key %q is not a leaf", key)
		}
		found := false
		for j := 0; j < current.NumField(); j++ {
			field := current.Type().Field(j)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get("toml"), ",")[0]
			if name != part {
				continue
			}
			current = current.Field(j)
			found = true
			break
		}
		if !found {
			return reflect.Value{}, fmt.Errorf("unknown key %q", key)
		}
		if i == len(parts)-1 {
			return current, nil
		}
	}
	return reflect.Value{}, fmt.Errorf("unknown key %q", key)
}
