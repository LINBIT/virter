package cliutils

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Parse takes a string of comma-separated k=v pairs and parses it to a struct
//
// For example the string "name=Titanic,year=1997" could be parsed into a struct like:
// 	type Film struct {
// 		Name   string `arg:"name"`
//		Year   int    `arg:"year"`
// 		Origin string `arg:"origin,Hollywood"
//	}
// Note that no "origin" was specified, so it will be set to the default "Hollywood".
// If no default value is given, the argument is required.
// Unused arguments are treated as errors.
func Parse(arg string, v interface{}) error {
	params, err := parseArgMap(arg)
	if err != nil {
		return fmt.Errorf("failed to parse %T: %w", v, err)
	}

	err = fillValues(params, v)
	if err != nil {
		return fmt.Errorf("failed to parse %T tags: %w", v, err)
	}

	return nil
}

func parseArgMap(str string) (map[string]string, error) {
	result := map[string]string{}

	kvpairs := strings.Split(str, ",")
	for _, kvpair := range kvpairs {
		if kvpair == "" {
			continue
		}
		kv := strings.Split(kvpair, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("malformed key/value pair '%s': expected exactly one '='", kvpair)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if key == "" {
			return nil, fmt.Errorf("malformed key/value pair '%s': key cannot be empty", kvpair)
		}

		result[key] = value
	}

	return result, nil
}

func fillValues(p map[string]string, v interface{}) error {
	t := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	switch t.Kind() {
	case reflect.Ptr:
		t = t.Elem()
		val = val.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := val.Field(i)
		if !fieldVal.CanSet() {
			continue
		}
		argParts := strings.SplitN(field.Tag.Get("arg"), ",", 2)
		name := argParts[0]
		if name == "" {
			name = field.Name
		}
		var defaultValue *string
		if len(argParts) == 2 {
			defaultValue = &argParts[1]
		}

		if _, ok := p[name]; !ok {
			if defaultValue == nil {
				return fmt.Errorf("missing required parameter '%s'", name)
			} else {
				p[name] = *defaultValue
			}
		}


		switch fieldVal.Kind() {
		case reflect.String:
			fieldVal.SetString(p[name])
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(p[name], 10, 0)
			if err != nil {
				return fmt.Errorf("failed to convert %s to int type: %w", p[name], err)
			}
			fieldVal.SetInt(n)
		case reflect.Uint,  reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(p[name], 10, 0)
			if err != nil {
				return fmt.Errorf("failed to convert %s to int type: %w", p[name], err)
			}
			fieldVal.SetUint(n)
		default:
			u, ok := fieldVal.Interface().(encoding.TextUnmarshaler)
			if !ok {
				if !fieldVal.CanAddr() {
					return fmt.Errorf("no known conversion from string to %s, maybe implement encoding.TextUnmarshaler", fieldVal.Type().String())
				}
				u, ok = fieldVal.Addr().Interface().(encoding.TextUnmarshaler)
				if !ok {
					return fmt.Errorf("no known conversion from string to %s, maybe implement encoding.TextUnmarshaler", fieldVal.Type().String())
				}
			}
			err := u.UnmarshalText([]byte(p[name]))
			if err != nil{
				return fmt.Errorf("failed to convert value %s: %w", p[name], err)
			}
		}

		delete(p, name)
	}

	if len(p) != 0 {
		return fmt.Errorf("unrecognized extra values: %v", p)
	}

	return nil
}
