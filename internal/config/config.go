package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Application struct {
	Postgres Postgres `envPrefix:"POSTGRES_"`
}

type Postgres struct {
	URL string `env:"URL"`
}

func Load() (Application, error) {
	var cfg Application
	err := env.ParseWithOptions(&cfg, env.Options{Prefix: "MOTO_"})
	if err != nil {
		return Application{}, fmt.Errorf("config parse error: %w", err)
	}
	return cfg, nil
}
