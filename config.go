// Package ConfigLoading provides utilities for loading configuration from environment variables and files
// using viper, with support for field masking and slog integration.
package ConfigLoading

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

// Logging enables slog.Warn outputs of the helper functions on some errors
var Logging bool = false

// ReadConfig is a helper function to read and parse config files into a config struct.
// It searches for configuration files (defaulting to .env or [APP_MODE].env) in the provided paths and the current directory.
// It also automatically binds environment variables to struct fields based on mapstructure tags.
func ReadConfig(config any, paths ...string) error {
	setupViper(paths)

	if err := loadPrimaryConfig(); err != nil {
		return err
	}

	if err := bindEnvForStruct(config); err != nil {
		return err
	}

	mergeAdditionalFiles(paths)

	return viper.Unmarshal(config)
}

func setupViper(paths []string) {
	viper.SetConfigType("env")
	for _, path := range paths {
		viper.AddConfigPath(path)
	}
	viper.AddConfigPath(".")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func loadPrimaryConfig() error {
	name := ".env"
	appMode := viper.GetString("APP_MODE")
	if appMode != "" {
		name = fmt.Sprintf("%s.env", appMode)
	}
	viper.SetConfigName(name)

	err := viper.ReadInConfig()
	if err == nil {
		return nil
	}

	if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		return nil // Ignore errors other than file not found for the first attempt
	}

	// If appMode-specific file not found, try .env
	if Logging {
		slog.Warn("App mode config file not found, looking for .env instead")
	}
	viper.SetConfigName(".env")
	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if Logging {
				slog.Warn("no config file found. Using environment variables and defaults.")
			}
		}
	}
	return nil
}

func mergeAdditionalFiles(paths []string) {
	files := []string{}
	for _, path := range paths {
		if !strings.HasSuffix(path, "/") {
			files = append(files, path)
		}
	}

	for _, file := range files {
		viper.SetConfigFile(file)
		viper.SetConfigType("env")
		if err := viper.MergeInConfig(); err != nil && Logging {
			slog.Warn("Failed to merge files.", "file", file, "err", err)
		}
	}
}

func bindEnvForStruct(configPtr any) error {
	reflectType := reflect.TypeOf(configPtr)
	if reflectType.Kind() != reflect.Ptr || reflectType.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("bindEnvForStruct expects pointer to struct, got %s", reflectType.Kind())
	}

	seen := map[string]struct{}{}
	keys := []string{}
	collectMapstructureKeys(reflectType.Elem(), seen, &keys)

	for _, key := range keys {
		_ = viper.BindEnv(key)
	}

	return nil
}

func collectMapstructureKeys(structType reflect.Type, seen map[string]struct{}, keys *[]string) {
	for fieldIndex := range structType.NumField() {
		structField := structType.Field(fieldIndex)

		if handled := handleEmbeddedStruct(structField, seen, keys); handled {
			continue
		}

		tag := structField.Tag.Get("mapstructure")
		if tag == "" || tag == ",squash" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		*keys = append(*keys, tag)
	}
}

func handleEmbeddedStruct(structField reflect.StructField, seen map[string]struct{}, keys *[]string) bool {
	if !structField.Anonymous {
		return false
	}

	if structField.Type.Kind() == reflect.Struct {
		collectMapstructureKeys(structField.Type, seen, keys)
		return true
	}

	if structField.Type.Kind() == reflect.Ptr && structField.Type.Elem().Kind() == reflect.Struct {
		collectMapstructureKeys(structField.Type.Elem(), seen, keys)
		return true
	}

	return false
}

// SafeForLogging returns a copy of the provided config with sensitive fields masked.
// Fields marked with the `masked:"true"` tag will be replaced with "********" (for strings and []byte)
// or their zero value (for other types).
func SafeForLogging(config any) (out any) {
	// masking based on `masked:"true"` struct tag
	val := reflect.ValueOf(config)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return config
	}

	// Create a pointer to a copy of the struct so we can modify it
	newStructPtr := reflect.New(val.Type())
	newStruct := newStructPtr.Elem()
	newStruct.Set(val)

	reflectType := newStruct.Type()

	for fieldIndex := range reflectType.NumField() {
		structField := reflectType.Field(fieldIndex)
		if structField.Tag.Get("masked") == "true" {
			fieldValue := newStruct.Field(fieldIndex)
			if !fieldValue.CanSet() {
				continue
			}

			switch fieldValue.Kind() {
			case reflect.String:
				if fieldValue.String() != "" {
					fieldValue.SetString("********")
				}
			case reflect.Slice:
				if structField.Type.Elem().Kind() == reflect.Uint8 && fieldValue.Len() > 0 { // []byte
					fieldValue.SetBytes([]byte("********"))
				} else {
					fieldValue.Set(reflect.Zero(fieldValue.Type()))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fieldValue.SetInt(0)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				fieldValue.SetUint(0)
			case reflect.Float32, reflect.Float64:
				fieldValue.SetFloat(0)
			case reflect.Complex64, reflect.Complex128:
				fieldValue.SetComplex(0)
			case reflect.Bool:
				fieldValue.SetBool(false)
			default:
				fieldValue.Set(reflect.Zero(fieldValue.Type()))
			}
		}
	}

	return newStruct.Interface()
}

// LogValue is a helper function to implement the slog.LogValuer interface.
// It allows structs to be logged securely by masking sensitive fields if safe is true.
//
// Example implementation:
//
//	func (conf MyConf) LogValue() slog.Value {
//		return ConfigLoading.LogValue(conf, true)
//	}
func LogValue(config any, safe bool) slog.Value {
	if safe {
		config = SafeForLogging(config)
	}

	reflectValue := reflect.ValueOf(config)
	reflectType := reflectValue.Type()

	attrs := make([]slog.Attr, 0, reflectType.NumField())
	for fieldIndex := range reflectType.NumField() {
		structField := reflectType.Field(fieldIndex)
		// skip unexported or explicitly non-logged fields
		if structField.PkgPath != "" {
			continue
		}

		// key preference: field name by default; use mapstructure only if explicitly requested via `logkey:"mapstructure"`
		key := structField.Name
		if structField.Tag.Get("logkey") == "mapstructure" {
			if tag := structField.Tag.Get("mapstructure"); tag != "" {
				key = tag
			}
		}

		fieldValue := reflectValue.Field(fieldIndex)

		// omit empty values
		switch fieldValue.Kind() {
		case reflect.String:
			if fieldValue.String() == "" {
				continue
			}
			attrs = append(attrs, slog.String(key, fieldValue.String()))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if fieldValue.Int() == 0 {
				continue
			}
			attrs = append(attrs, slog.Int64(key, fieldValue.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if fieldValue.Uint() == 0 {
				continue
			}
			attrs = append(attrs, slog.Uint64(key, fieldValue.Uint()))
		case reflect.Bool:
			if !fieldValue.Bool() {
				continue
			}
			attrs = append(attrs, slog.Bool(key, fieldValue.Bool()))
		case reflect.Float32, reflect.Float64:
			if fieldValue.Float() == 0 {
				continue
			}
			attrs = append(attrs, slog.Float64(key, fieldValue.Float()))
		case reflect.Slice, reflect.Array:
			if fieldValue.Len() == 0 {
				continue
			}
			attrs = append(attrs, slog.Any(key, fieldValue.Interface()))
		case reflect.Map:
			if fieldValue.Len() == 0 {
				continue
			}
			attrs = append(attrs, slog.Any(key, fieldValue.Interface()))
		case reflect.Struct:
			attrs = append(attrs, slog.Any(key, fieldValue.Interface()))
		case reflect.Pointer, reflect.Interface:
			if fieldValue.IsNil() {
				continue
			}
			attrs = append(attrs, slog.Any(key, fieldValue.Interface()))
		default:
			attrs = append(attrs, slog.Any(key, fieldValue.Interface()))
		}
	}

	return slog.GroupValue(attrs...)
}

// String returns a string representation of the config struct, useful for implementing the fmt.Stringer interface.
// It can optionally mask sensitive fields and omits zero-value fields by default.
func String(config any, safe bool) string {
	if safe {
		config = SafeForLogging(config)
	}
	reflectValue := reflect.ValueOf(config)
	if reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	reflectType := reflectValue.Type()

	var builder strings.Builder
	typeName := reflectType.Name()
	builder.WriteString(typeName)
	builder.WriteString("{")
	firstField := true
	for fieldIndex := range reflectType.NumField() {
		structField := reflectType.Field(fieldIndex)
		if structField.PkgPath != "" {
			continue
		}

		// key preference: field name by default; use mapstructure only if explicitly requested via `logkey:"mapstructure"`
		key := structField.Name
		if structField.Tag.Get("logkey") == "mapstructure" {
			if tag := structField.Tag.Get("mapstructure"); tag != "" {
				key = tag
			}
		}

		fieldValue := reflectValue.Field(fieldIndex)

		// omit empty values
		omit := false
		switch fieldValue.Kind() {
		case reflect.String:
			omit = fieldValue.String() == ""
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			omit = fieldValue.Int() == 0
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			omit = fieldValue.Uint() == 0
		case reflect.Bool:
			omit = !fieldValue.Bool()
		case reflect.Float32, reflect.Float64:
			omit = fieldValue.Float() == 0
		case reflect.Complex64, reflect.Complex128:
			omit = fieldValue.Complex() == 0
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
			omit = reflect.DeepEqual(fieldValue.Interface(), reflect.Zero(fieldValue.Type()).Interface())
		case reflect.Pointer, reflect.Interface:
			omit = fieldValue.IsNil()
		}
		if omit {
			continue
		}

		if !firstField {
			builder.WriteString(", ")
		}
		firstField = false
		builder.WriteString(key)
		builder.WriteString("=")
		val := fieldValue.Interface()
		if fieldValue.Kind() == reflect.Pointer && !fieldValue.IsNil() {
			val = fieldValue.Elem().Interface()
		}
		builder.WriteString(fmt.Sprint(val))
	}
	builder.WriteString("}")
	return builder.String()
}
