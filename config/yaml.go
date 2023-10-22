package config

import (
	"log"
	"net"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

// Config config
type Config struct {
	General      GeneralConfig         `yaml:"general"`
	HTTPServers  []VirtualServerConfig `yaml:"http"`
	HTTPSServers []VirtualServerConfig `yaml:"https"`
}

type GeneralConfig struct {
	CertsPath  string     `yaml:"certspath"`
	IP         string     `yaml:"ip"`
	Port       string     `yaml:"port"`
	SecurePort string     `yaml:"secureport"`
	Hosts      []KeyValue `yaml:"hosts"`
	Ports      []KeyValue `yaml:"ports"`
}

type KeyValue struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// VirtualServerConfig virtual host config
type VirtualServerConfig struct {
	Domain     string `yaml:"domain"`
	Host       string `yaml:"host"`
	Port       string `yaml:"port"`
	Cert       string `yaml:"cert"`
	Key        string `yaml:"key"`
	Prefix     string `yaml:"prefix"`
	SecureMode bool   `yaml:"securemode"`
}

func (c *VirtualServerConfig) Addr() string {
	host, ok := name2host[c.Host]
	if !ok {
		if net.ParseIP(c.Host) == nil {
			log.Fatalf("can not find the value of host %s", host)
		} else {
			host = c.Host
		}
	}

	port, ok := name2port[c.Port]
	if !ok {
		p, err := strconv.Atoi(c.Port)
		if err != nil || p <= 0 || p >= 65535 {
			log.Fatalf("can not find the value of host %s", port)
		} else {
			port = c.Port
		}
	}

	return host + ":" + port
}

var conf = &Config{}

// Read read config
func Read(path string) error {

	fd, err := os.Open(path)

	if err != nil {
		return err
	}

	yaml.NewDecoder(fd).Decode(conf)
	log.Printf("%v", conf)

	buildConfig(conf)

	return nil
}

