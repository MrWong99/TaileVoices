package config

import (
	"context"
	"os"

	"gopkg.in/yaml.v3"
)

var path string

func init() {
	if env, ok := os.LookupEnv("CONFIG_PATH"); ok {
		path = env
	} else {
		path = "tailevoices-bot.yml"
	}
}

// Path returns the absolute or relative path of the set configuration file path.
// The path can be set via the CONFIG_PATH environment variable and defaults to
// tailevoices-bot.yml if not set.
func Path() string {
	return path
}

// Agent configuration options.
type Agent struct {
	// Token to access the bot API.
	Token string
}

// OpenAI configuration options.
type OpenAI struct {
	// Token to access the Discord platform API.
	Token string
}

type SpeechToText struct {
	ModelPath string `yaml:"modelPath"`
}

// App encapsulates the entire application config.
type App struct {
	OpenAI       OpenAI       `yaml:"openAI"`
	Agent        Agent        `yaml:"agent"`
	SpeechToText SpeechToText `yaml:"speechToText"`
}

type ContextKey uint

const (
	// TokenKey is the key used to embed the token in a context value once AddTokenToContext is called.
	TokenKey ContextKey = iota
)

// AddTokenToContext returns a new context that has an additional value set.
// The key of the value is config.TokenKey and the value is the configured token.
func (a *Agent) AddTokenToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, TokenKey, a.Token)
}

// Load the configuration that is stored in Path().
func Load() (cfg *App, err error) {
	cfg = &App{}
	var f *os.File
	f, err = os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	err = yaml.NewDecoder(f).Decode(cfg)
	return
}
