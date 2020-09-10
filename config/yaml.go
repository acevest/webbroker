package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config config
type Config struct {
	HTTPHosts  []virtualHostConfig `yaml:"http"`
	HTTPSHosts []virtualHostConfig `yaml:"https"`
}

type virtualHostConfig struct {
	Domain string `yaml:"domain"`
	Host   string `yaml:"host"`
}

// Read read config
func Read(path string) (*Config, error) {

	var cfg = &Config{}
	fd, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	yaml.NewDecoder(fd).Decode(cfg)

	return cfg, nil
}
