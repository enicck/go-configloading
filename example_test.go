package ConfigLoading_test

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/enicck/go-configloading"
)

type MyConfig struct {
	AppHost  string `mapstructure:"APP_HOST"`
	AppPort  int    `mapstructure:"APP_PORT"`
	DBPass   string `mapstructure:"DB_PASS" masked:"true"`
	LogLevel string `mapstructure:"LOG_LEVEL"`
}

func (c MyConfig) LogValue() slog.Value {
	return ConfigLoading.LogValue(c, true)
}

func (c MyConfig) String() string {
	return ConfigLoading.String(c, true)
}

func ExampleReadConfig() {
	// Simulate environment variables
	os.Setenv("APP_PORT", "8080")
	os.Setenv("APP_HOST", "localhost")
	defer os.Unsetenv("APP_PORT")
	defer os.Unsetenv("APP_HOST")

	// Create a dummy .env file
	os.WriteFile(".env", []byte("LOG_LEVEL=info\nDB_PASS=secret"), 0644)
	defer os.Remove(".env")

	var conf MyConfig
	err := ConfigLoading.ReadConfig(&conf)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Host: %s, Port: %d, Level: %s", conf.AppHost, conf.AppPort, conf.LogLevel)
	// Output: Host: localhost, Port: 8080, Level: info
}

func ExampleSafeForLogging() {
	conf := MyConfig{
		DBPass: "super-secret-password",
	}

	masked := ConfigLoading.SafeForLogging(conf).(MyConfig)
	fmt.Println(masked.DBPass)
	// Output: ********
}

func ExampleString() {
	conf := MyConfig{
		AppHost: "localhost",
		DBPass:  "secret-password",
	}

	// safe=true will mask fields with `masked:"true"`
	fmt.Println(ConfigLoading.String(conf, true))
	// Output: MyConfig{AppHost=localhost, DBPass=********}
}

func ExampleLogValue() {
	conf := MyConfig{
		AppHost: "localhost",
		DBPass:  "password123",
	}

	// When using slog, LogValue is called automatically if implemented
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Remove time for deterministic output
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))

	logger.Info("starting app", "config", conf)
	// Output: level=INFO msg="starting app" config.AppHost=localhost config.DBPass=********
}
