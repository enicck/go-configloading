# go-configloading

`go-configloading` is a lightweight Go utility for loading configuration from environment variables and `.env` files using `viper`, with built-in support for secure logging through field masking and `log/slog` integration.

## Features

- **Environment-aware Loading**: Automatically loads `.env` or `[APP_MODE].env` files based on the `APP_MODE` environment variable.
- **Recursive Env Binding**: Automatically binds environment variables to nested struct fields using `mapstructure` tags.
- **Secure Logging**:
    - Mask sensitive fields (e.g., passwords, tokens) using the `masked:"true"` struct tag.
    - Specialized masking for `string` and `[]byte` ("********").
    - Zero-value resetting for other sensitive types.
- **`log/slog` Integration**: Implements helper for `slog.LogValuer` to ensure safe logging of configuration objects.
- **Clean String Representation**: Custom `String()` helper that omits zero values and respects masking for easy debugging.

## Installation

```bash
go get github.com/enicck/go-configloading
```

## Usage

### 1. Define your Configuration

Use `mapstructure` tags for environment variable mapping and `masked:"true"` for sensitive fields.

```go
type MyConfig struct {
    AppPort  int    `mapstructure:"APP_PORT"`
    DBPass   string `mapstructure:"DB_PASS" masked:"true"`
    LogLevel string `mapstructure:"LOG_LEVEL"`
}

// Optional: Implement slog.LogValuer for secure logging
func (c MyConfig) LogValue() slog.Value {
    return ConfigLoading.LogValue(c, true)
}

// Optional: Implement fmt.Stringer
func (c MyConfig) String() string {
    return ConfigLoading.String(c, true)
}
```

### 2. Load Configuration

```go
var conf MyConfig
err := ConfigLoading.ReadConfig(&conf)
if err != nil {
    log.Fatal(err)
}
```

### 3. Safe Logging

```go
// Using slog (calls LogValue automatically)
slog.Info("app started", "config", conf)

// Manual masking
safeConf := ConfigLoading.SafeForLogging(conf).(MyConfig)
fmt.Println(safeConf.DBPass) // Output: ********
```

## Struct Tags

- `mapstructure`: Used by `viper` to map environment variables/file keys to fields.
- `masked:"true"`: Marks a field as sensitive. It will be masked by `SafeForLogging`, `LogValue`, and `String`.
- `logkey:"mapstructure"`: Tells `LogValue` or `String` to use the `mapstructure` tag value as the key in the output instead of the field name.

## Examples

Check [example_test.go](./example_test.go) for more detailed usage examples.
