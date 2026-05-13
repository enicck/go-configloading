package ConfigLoading

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

type EmbeddedConfig struct {
	EmbeddedField string `mapstructure:"EMBEDDED_FIELD"`
}

type PtrEmbeddedConfig struct {
	PtrEmbeddedField string `mapstructure:"PTR_EMBEDDED_FIELD"`
}

type TestConfig struct {
	EmbeddedConfig    `mapstructure:",squash"`
	PtrEmbeddedConfig `mapstructure:",squash"`
	SecretString      string         `mapstructure:"SECRET_STRING" masked:"true"`
	PublicString      string         `mapstructure:"PUBLIC_STRING"`
	Duplicate         string         `mapstructure:"PUBLIC_STRING"`
	SecretInt         int            `mapstructure:"SECRET_INT" masked:"true"`
	PublicInt         int            `mapstructure:"PUBLIC_INT"`
	SecretUint        uint           `mapstructure:"SECRET_UINT" masked:"true"`
	SecretBytes       []byte         `mapstructure:"SECRET_BYTES" masked:"true"`
	SecretBool        bool           `masked:"true"`
	SecretFloat       float64        `masked:"true"`
	SecretComplex     complex128     `masked:"true"`
	SecretMap         map[string]int `masked:"true"`
	SecretSlice       []string       `masked:"true"`
	SecretArray       [2]int         `masked:"true"`
	SecretPtr         *int           `masked:"true"`
	unexported        string         `masked:"true"`
}

func TestSafeForLogging(testingT *testing.T) {
	val := 10
	tests := []struct {
		name           string
		config         any
		expectedConfig any
	}{
		{
			name: "Struct with masked fields",
			config: TestConfig{
				SecretString:  "secret",
				PublicString:  "public",
				SecretInt:     123,
				PublicInt:     456,
				SecretUint:    789,
				SecretBytes:   []byte("secretbytes"),
				SecretBool:    true,
				SecretFloat:   1.23,
				SecretComplex: complex(1, 2),
				SecretMap:     map[string]int{"key": 1},
				SecretSlice:   []string{"a", "b"},
				SecretArray:   [2]int{1, 2},
				SecretPtr:     &val,
				unexported:    "don't mask me",
			},
			expectedConfig: TestConfig{
				SecretString:  "********",
				PublicString:  "public",
				SecretInt:     0,
				PublicInt:     456,
				SecretUint:    0,
				SecretBytes:   []byte("********"),
				SecretBool:    false,
				SecretFloat:   0,
				SecretComplex: 0,
				SecretMap:     nil,
				SecretSlice:   nil,
				SecretArray:   [2]int{0, 0},
				SecretPtr:     nil,
				unexported:    "don't mask me",
			},
		},
		{
			name: "Pointer to struct",
			config: &TestConfig{
				SecretString: "secret",
			},
			expectedConfig: TestConfig{
				SecretString: "********",
			},
		},
		{
			name:           "Non-struct type",
			config:         "just a string",
			expectedConfig: "just a string",
		},
		{
			name: "Empty values should not be changed",
			config: TestConfig{
				SecretString: "",
				SecretBytes:  nil,
			},
			expectedConfig: TestConfig{
				SecretString: "",
				SecretBytes:  nil,
			},
		},
	}
	for _, test := range tests {
		testingT.Run(test.name, func(testingT *testing.T) {
			result := SafeForLogging(test.config)
			if !reflect.DeepEqual(test.expectedConfig, result) {
				testingT.Errorf("%s: SafeForLogging(%#v) => %#v, want %#v", test.name, test.config, result, test.expectedConfig)
			}
		})
	}
}

func TestReadConfig(t *testing.T) {
	tests := []struct {
		name           string
		defaultConfig  TestConfig
		envFileContent string
		appMode        string
		expectedConfig TestConfig
		osEnvValue     string
		errorExpected  bool
	}{
		{
			name: "Use default values",
			defaultConfig: TestConfig{
				SecretString: "secret",
			},
			envFileContent: `PUBLIC_STRING=public`,
			expectedConfig: TestConfig{
				SecretString: "secret",
				PublicString: "public",
				Duplicate:    "public",
			},
		},
		{
			name: "ensure default overwrite",
			defaultConfig: TestConfig{
				SecretString: "defaultVal",
			},
			envFileContent: `SECRET_STRING=secret`,
			expectedConfig: TestConfig{
				SecretString: "secret",
			},
		},
		{
			name:           "APP_MODE set, but file missing (fallback to .env)",
			appMode:        "prod",
			envFileContent: `PUBLIC_STRING=prod-env`,
			expectedConfig: TestConfig{
				PublicString: "prod-env",
				Duplicate:    "prod-env",
			},
		},
		{
			name:           "APP_MODE set and file exists",
			appMode:        "dev",
			envFileContent: `PUBLIC_STRING=dev-env`,
			expectedConfig: TestConfig{
				PublicString: "dev-env",
				Duplicate:    "dev-env",
			},
		},
		{
			name:           "Environment variables override file",
			osEnvValue:     "env-override",
			envFileContent: `PUBLIC_STRING=file-value`,
			expectedConfig: TestConfig{
				PublicString: "env-override",
				Duplicate:    "env-override",
			},
		},
		{
			name:           "No config file found",
			envFileContent: "",
			expectedConfig: TestConfig{},
		},
		{
			name: "Paths with and without trailing slashes",
			defaultConfig: TestConfig{
				PublicString: "default",
			},
			expectedConfig: TestConfig{
				PublicString: "extra",
				Duplicate:    "extra",
			},
		},
		{
			name:           "Invalid config struct (not a pointer)",
			errorExpected:  true,
			defaultConfig:  TestConfig{},
			expectedConfig: TestConfig{},
		},
		{
			name:           "Unmarshal failure",
			errorExpected:  true,
			defaultConfig:  TestConfig{},
			expectedConfig: TestConfig{},
		},
		{
			name:           "Embedded structs and duplicate tags",
			envFileContent: "EMBEDDED_FIELD=embedded\nPTR_EMBEDDED_FIELD=ptr-embedded\nPUBLIC_STRING=public",
			defaultConfig: TestConfig{
				PtrEmbeddedConfig: PtrEmbeddedConfig{},
			},
			expectedConfig: TestConfig{
				EmbeddedConfig: EmbeddedConfig{
					EmbeddedField: "embedded",
				},
				PtrEmbeddedConfig: PtrEmbeddedConfig{
					PtrEmbeddedField: "ptr-embedded",
				},
				PublicString: "public",
				Duplicate:    "public",
			},
		},
		{
			name:           "Duplicate tags in struct",
			envFileContent: "PUBLIC_STRING=first",
			defaultConfig:  TestConfig{},
			expectedConfig: TestConfig{
				PublicString: "first",
				Duplicate:    "first",
			},
		},
		{
			name:           "APP_MODE set to prod but prod.env missing, fallback to .env",
			appMode:        "prod",
			envFileContent: "PUBLIC_STRING=fallback",
			expectedConfig: TestConfig{
				PublicString: "fallback",
				Duplicate:    "fallback",
			},
		},
		{
			name:           "Corrupted config file",
			envFileContent: "INVALID=JSON=STUFF", // or just something that viper.ReadInConfig() might choke on if it's not a valid env file
			errorExpected:  false,                // viper.ReadInConfig() for ENV type usually doesn't fail on "invalid" content unless it's really bad or it's another type
		},
		{
			name:           "Viper Merge failure",
			errorExpected:  false, // It logs a warning but doesn't return error
			defaultConfig:  TestConfig{},
			expectedConfig: TestConfig{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset viper for each test case to avoid interference
			viper.Reset()

			if test.appMode != "" {
				os.Setenv("APP_MODE", test.appMode)
				defer os.Unsetenv("APP_MODE")
			}
			if test.osEnvValue != "" {
				os.Setenv("PUBLIC_STRING", test.osEnvValue)
				defer os.Unsetenv("PUBLIC_STRING")
			}

			// Enable logging for some tests to cover slog.Warn calls
			Logging = true
			defer func() { Logging = false }()

			// write env files
			var envFileName string
			if test.appMode != "" && test.name != "APP_MODE set to prod but prod.env missing, fallback to .env" {
				envFileName = fmt.Sprintf("%s.env", test.appMode)
			} else {
				envFileName = ".env"
			}

			if test.envFileContent != "" {
				_ = os.WriteFile(envFileName, []byte(test.envFileContent), 0644)
				defer os.Remove(envFileName)
			}

			// load config
			config := test.defaultConfig
			var err error
			if test.name == "Paths with and without trailing slashes" {
				// Special case for testing paths
				_ = os.WriteFile("extra.env", []byte("PUBLIC_STRING=extra"), 0644)
				defer os.Remove("extra.env")
				err = ReadConfig(&config, "./extra.env", "ignored/")
			} else if test.name == "Corrupted config file" {
				_ = os.WriteFile("corrupted.env", []byte("KEY=VALUE"), 0000) // No permissions
				defer os.Remove("corrupted.env")
				os.Setenv("APP_MODE", "corrupted")
				err = ReadConfig(&config)
				os.Unsetenv("APP_MODE")
			} else if test.name == "Invalid config struct (not a pointer)" {
				err = ReadConfig(config) // pass by value instead of pointer
			} else if test.name == "Unmarshal failure" {
				err = ReadConfig((*TestConfig)(nil))
			} else if test.name == "Viper Merge failure" {
				err = ReadConfig(&config, "non-existent-file")
			} else {
				err = ReadConfig(&config)
			}

			if err != nil && !test.errorExpected {
				t.Errorf("Error reading config: %v", err)
			}
			if err == nil && test.errorExpected {
				t.Errorf("Expected error but got nil")
			}

			if !test.errorExpected && !reflect.DeepEqual(test.expectedConfig, config) {
				t.Errorf("ReadConfig(%#v) => \n%#v\nwant\n%#v", test.defaultConfig, config, test.expectedConfig)
			}
		})
	}
}

func TestLogValue(t *testing.T) {
	type LogTestConfig struct {
		Name       string `logkey:"mapstructure" mapstructure:"app_name"`
		Version    string
		Secret     string `masked:"true"`
		Port       int
		unexported string
	}

	tests := []struct {
		name     string
		config   any
		safe     bool
		expected map[string]any
	}{
		{
			name: "Unsafe logging - all fields except unexported",
			config: LogTestConfig{
				Name:       "MyApp",
				Version:    "1.0.0",
				Secret:     "password",
				Port:       8080,
				unexported: "private",
			},
			safe: false,
			expected: map[string]any{
				"app_name": "MyApp",
				"Version":  "1.0.0",
				"Secret":   "password",
				"Port":     int64(8080),
			},
		},
		{
			name: "Safe logging - secret masked",
			config: LogTestConfig{
				Name:    "MyApp",
				Version: "1.0.0",
				Secret:  "password",
				Port:    8080,
			},
			safe: true,
			expected: map[string]any{
				"app_name": "MyApp",
				"Version":  "1.0.0",
				"Secret":   "********",
				"Port":     int64(8080),
			},
		},
		{
			name: "Omit zero values",
			config: LogTestConfig{
				Name: "MyApp",
			},
			safe: false,
			expected: map[string]any{
				"app_name": "MyApp",
			},
		},
		{
			name: "All supported types and pointer/interface",
			config: struct {
				Int       int
				Uint      uint
				Bool      bool
				Float     float64
				Slice     []string
				Array     [1]int
				Map       map[string]int
				Ptr       *int
				Interface any
				Other     complex128
			}{
				Int:       1,
				Uint:      2,
				Bool:      true,
				Float:     3.4,
				Slice:     []string{"a"},
				Array:     [1]int{5},
				Map:       map[string]int{"b": 6},
				Ptr:       new(int),
				Interface: "test",
				Other:     complex(1, 2),
			},
			safe: false,
			expected: map[string]any{
				"Int":       int64(1),
				"Uint":      uint64(2),
				"Bool":      true,
				"Float":     float64(3.4),
				"Slice":     []string{"a"},
				"Array":     [1]int{5},
				"Map":       map[string]int{"b": 6},
				"Ptr":       new(int),
				"Interface": "test",
				"Other":     complex(1, 2),
			},
		},
		{
			name: "Embedded struct",
			config: struct {
				LogTestConfig
				Extra string
			}{
				LogTestConfig: LogTestConfig{Name: "Embedded"},
				Extra:         "Bonus",
			},
			safe: false,
			expected: map[string]any{
				"LogTestConfig": LogTestConfig{Name: "Embedded"},
				"Extra":         "Bonus",
			},
		},
		{
			name: "logkey with missing mapstructure tag",
			config: struct {
				NoTag string `logkey:"mapstructure"`
			}{
				NoTag: "should use field name",
			},
			safe: false,
			expected: map[string]any{
				"NoTag": "should use field name",
			},
		},
		{
			name: "logkey with empty mapstructure tag",
			config: struct {
				EmptyTag string `logkey:"mapstructure" mapstructure:""`
			}{
				EmptyTag: "should use field name",
			},
			safe: false,
			expected: map[string]any{
				"EmptyTag": "should use field name",
			},
		},
		{
			name: "default case (chan)",
			config: struct {
				Chan chan int
			}{
				Chan: nil,
			},
			safe: false,
			expected: map[string]any{
				"Chan": (chan int)(nil),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			val := LogValue(test.config, test.safe)
			if val.Kind() != slog.KindGroup {
				t.Fatalf("expected GroupValue, got %v", val.Kind())
			}

			attrs := val.Group()
			if len(attrs) != len(test.expected) {
				t.Errorf("expected %d attributes, got %d", len(test.expected), len(attrs))
			}

			for _, attr := range attrs {
				expectedVal, ok := test.expected[attr.Key]
				if !ok {
					t.Errorf("unexpected attribute key: %s", attr.Key)
					continue
				}
				if !reflect.DeepEqual(expectedVal, attr.Value.Any()) {
					t.Errorf("key %s: expected %v, got %v", attr.Key, expectedVal, attr.Value.Any())
				}
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		config   any
		safe     bool
		expected string
	}{
		{
			name: "Safe logging with masking",
			config: struct {
				Secret string `masked:"true"`
				Public string
			}{
				Secret: "password",
				Public: "username",
			},
			safe:     true,
			expected: "{Secret=********, Public=username}",
		},
		{
			name: "Unsafe logging without masking",
			config: struct {
				Secret string `masked:"true"`
				Public string
			}{
				Secret: "password",
				Public: "username",
			},
			safe:     false,
			expected: "{Secret=password, Public=username}",
		},
		{
			name: "Omit empty values",
			config: struct {
				Full  string
				Empty string
				Zero  int
			}{
				Full:  "value",
				Empty: "",
				Zero:  0,
			},
			safe:     false,
			expected: "{Full=value}",
		},
		{
			name: "Custom log key via logkey tag",
			config: struct {
				Field string `logkey:"mapstructure" mapstructure:"app_name"`
			}{
				Field: "MyApp",
			},
			safe:     false,
			expected: "{app_name=MyApp}",
		},
		{
			name: "All supported types",
			config: struct {
				Int       int
				Uint      uint
				Bool      bool
				Float     float64
				Slice     []string
				Array     [1]int
				Map       map[string]int
				Ptr       *int
				Interface any
			}{
				Int:       1,
				Uint:      2,
				Bool:      true,
				Float:     3.4,
				Slice:     []string{"a"},
				Array:     [1]int{5},
				Map:       map[string]int{"b": 6},
				Ptr:       new(10),
				Interface: "test",
			},
			safe:     false,
			expected: "{Int=1, Uint=2, Bool=true, Float=3.4, Slice=[a], Array=[5], Map=map[b:6], Ptr=10, Interface=test}",
		},
		{
			name: "Omit nil pointer and interface",
			config: struct {
				Ptr       *int
				Interface any
			}{
				Ptr:       nil,
				Interface: nil,
			},
			safe:     false,
			expected: "{}",
		},
		{
			name: "Skip unexported fields",
			config: struct {
				Public     string
				unexported string
			}{
				Public:     "visible",
				unexported: "hidden",
			},
			safe:     false,
			expected: "{Public=visible}",
		},
		{
			name: "Named struct",
			config: TestConfig{
				PublicString: "named",
				Duplicate:    "other",
			},
			safe:     false,
			expected: "TestConfig{PublicString=named, Duplicate=other}",
		},
		{
			name: "Pointer to named struct",
			config: &TestConfig{
				PublicString: "pointer",
				Duplicate:    "other",
			},
			safe:     false,
			expected: "TestConfig{PublicString=pointer, Duplicate=other}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := String(test.config, test.safe)
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}
