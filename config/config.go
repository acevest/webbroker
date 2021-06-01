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

func getVirtualServerAddr(host string, hostCfgs map[string]VirtualServerConfig) (string, error) {
	host = strings.TrimSpace(host)
	c, ok := hostCfgs[host]
	if !ok {
		for k, v := range hostCfgs {
			if len(k) < len(host) {
				if host[len(host)-len(k):] == k {
					return v.Addr(), nil
				}
			}
		}
		return "", fmt.Errorf("can not find %v", host)
	}

	return c.Addr(), nil
}

func GetVirtualHTTPServerAddr(host string) (string, error) {
	return getVirtualServerAddr(host, virtualHTTPServers)
}

func GetVirtualHTTPSServerAddr(host string) (string, error) {
	return getVirtualServerAddr(host, virtualHTTPSServers)
}

func GetAllHTTPSServer() []VirtualServerConfig {
	var s []VirtualServerConfig
	for _, v := range virtualHTTPSServers {
		s = append(s, v)
	}

	return s
}

func buildConfig(cfg *Config) {

	for _, cfg := range cfg.General.Hosts {
		fmt.Printf("general host %s = %s ", cfg.Name, cfg.Value)
		name2host[cfg.Name] = cfg.Value
	}

	for _, cfg := range cfg.General.Ports {
		fmt.Printf("general port %s = %s ", cfg.Name, cfg.Value)
		name2port[cfg.Name] = cfg.Value
	}

	for _, cfg := range cfg.HTTPServers {
		log.Printf("http: %v\n", cfg)
		virtualHTTPServers[cfg.Domain] = cfg
	}
	for _, cfg := range cfg.HTTPSServers {
		log.Printf("https: %v\n", cfg)
		virtualHTTPSServers[cfg.Domain] = cfg
	}

}
