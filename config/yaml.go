package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config config
type Config struct {
	HTTPHosts  []VirtualHostConfig `yaml:"http"`
	HTTPSHosts []VirtualHostConfig `yaml:"https"`
}

// VirtualHostConfig virtual host config
type VirtualHostConfig struct {
	Domain string `yaml:"domain"`
	Host   string `yaml:"host"`
	Cert   string `yaml:"cert"`
	Key    string `yaml:"key"`
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
