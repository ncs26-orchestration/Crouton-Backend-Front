package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Port        string `env:"PORT" envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`
	AgentURL    string `env:"AGENT_URL" envDefault:"http://agent:8000"`
	Environment string `env:"APP_ENV" envDefault:"development"`
	JWTSecret   string `env:"JWT_SECRET" envDefault:"change-me-in-production"`

	// OrchStepDelayMS paces the orchestration engine between node steps so
	// progression is watchable in the UI. ~600ms reads as a live, deliberate
	// pipeline (10 nodes ≈ a few seconds) rather than an instant flip.
	OrchStepDelayMS int `env:"ORCH_STEP_DELAY_MS" envDefault:"600"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}
