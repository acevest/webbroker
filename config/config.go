/*
 * ------------------------------------------------------------------------
 *   File Name: config.go
 *      Author: Zhao Yanbai
 *              2021-06-01 18:59:06 Tuesday CST
 * Description: none
 * ------------------------------------------------------------------------
 */

package config

import (
	"fmt"
	"log"
	"strings"
)

var name2host = map[string]string{}
var name2port = map[string]string{}

var virtualHTTPServers = map[string]VirtualServerConfig{}
var virtualHTTPSServers = map[string]VirtualServerConfig{}

func GetVirtualHTTPServerAddr(host, path string) (*VirtualServerConfig, error) {
	host = strings.TrimSpace(host)
	var cfg *VirtualServerConfig
	for _, c := range Conf.HTTPServers {
		log.Printf("http: %v\n", c)
		log.Printf("a:%v b:%v c:%v d:%v", host, path, c.Domain, c.Prefix)
		if host == c.Domain && len(c.Prefix) == 0 {
			cfg = &c
			// no break
		} else if host == c.Domain && len(c.Prefix) > 0 && strings.HasPrefix(path, c.Prefix) {
			log.Printf("has prefix %v %v", path, c.Prefix)
			cfg = &c
			break
		}
	}

	return cfg, nil

}

func GetVirtualHTTPSServerAddr(host, path string) (*VirtualServerConfig, error) {
	host = strings.TrimSpace(host)
	var cfg VirtualServerConfig
	for _, c := range Conf.HTTPSServers {
		log.Printf("http: %v\n", c)
		log.Printf("a:%v b:%v c:%v d:%v %v", host, path, c.Domain, c.Prefix, len(c.Prefix))
		if host == c.Domain && len(c.Prefix) == 0 {
			log.Printf("A")
			cfg = c
			// no break
		} else if host == c.Domain && len(c.Prefix) > 0 && strings.HasPrefix(path, c.Prefix) {
			log.Printf("B")
			log.Printf("has prefix %v %v", path, c.Prefix)
			cfg = c
			break
		}
	}

	log.Printf("choose %v", cfg)
	return &cfg, nil
	//return nil, fmt.Errorf("can not find %v", host)
}

func GetAllHTTPSServer() []VirtualServerConfig {
	var s []VirtualServerConfig
	for _, v := range virtualHTTPSServers {
		s = append(s, v)
	}

	return s
}

var IP string
var Port string
var SecurePort string
var CertsPath string

func buildConfig(cfg *Config) {
	CertsPath = cfg.General.CertsPath

	if cfg.General.Port == "" {
		log.Printf("fuck port empty")
		Port = "80"
	} else {
		Port = cfg.General.Port
	}

	IP = cfg.General.IP
	SecurePort = cfg.General.SecurePort
	log.Printf("secure port: %v", SecurePort)

	for _, cfg := range cfg.General.Hosts {
		fmt.Printf("general host %s = %s\n", cfg.Name, cfg.Value)
		name2host[cfg.Name] = cfg.Value
	}

	for _, cfg := range cfg.General.Ports {
		fmt.Printf("general port %s = %s\n", cfg.Name, cfg.Value)
		name2port[cfg.Name] = cfg.Value
	}

	for i, c := range Conf.HTTPSServers {
		if v, ok := name2port[c.Port]; ok {
			Conf.HTTPSServers[i].Port = v
		}
	}

	for _, cfg := range cfg.HTTPServers {
		log.Printf(">http: %v\n", cfg)
		virtualHTTPServers[cfg.Domain] = cfg
	}
	for _, cfg := range cfg.HTTPSServers {
		log.Printf("https: %v\n", cfg)
		virtualHTTPSServers[cfg.Domain] = cfg
	}

}
