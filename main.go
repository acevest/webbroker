package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"webbroker/config"
)

var virtualHTTPHosts = map[string]config.VirtualHostConfig{}
var virtualHTTPSHosts = map[string]config.VirtualHostConfig{}

func getVirtualHTTPHostAddr(host string) (string, error) {
	host = strings.TrimSpace(host)
	c, ok := virtualHTTPHosts[host]
	if !ok {
		for k, v := range virtualHTTPHosts {
			if len(k) < len(host) {
				if host[len(host)-len(k):] == k {
					return v.Host, nil
				}
			}
		}
		return "", fmt.Errorf("can not find %v", host)
	}

	return c.Host, nil
}

func getVirtualHTTPSHostAddr(host string) (string, error) {
	host = strings.TrimSpace(host)
	c, ok := virtualHTTPSHosts[host]
	if !ok {
		for k, v := range virtualHTTPSHosts {
			if len(k) < len(host) {
				if host[len(host)-len(k):] == k {
					return v.Host, nil
				}
			}

		}
		return "", fmt.Errorf("can not find %v", host)
	}

	return c.Host, nil
}
func main() {
	var cfgPath string
	var forceHTTPS bool
	flag.StringVar(&cfgPath, "config", "config.yaml", "config file path")
	flag.BoolVar(&forceHTTPS, "forcehttps", false, "use https only")
	flag.Parse()
	cfg, err := config.Read(cfgPath)
	if err != nil {
		log.Fatalf("read config file failed, err: %v", err)
	}

	for _, cfg := range cfg.HTTPHosts {
		fmt.Printf("http: %v\n", cfg)
		virtualHTTPHosts[cfg.Domain] = cfg
	}
	for _, cfg := range cfg.HTTPSHosts {
		fmt.Printf("https: %v\n", cfg)
		virtualHTTPSHosts[cfg.Domain] = cfg
	}

	if forceHTTPS {
		go httpForceHTTPS()
	} else {
		go httpServer()
	}
	httpsServer()
}

func httpsServer() {
	tlsCfg := &tls.Config{}
	for _, cfg := range virtualHTTPSHosts {
		cert, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
		if err != nil {
			log.Fatal(err)
		}
		tlsCfg.Certificates = append(tlsCfg.Certificates, cert)
	}
	tlsCfg.BuildNameToCertificate()
	tlsCfg.Time = time.Now
	tlsCfg.Rand = rand.Reader

	listener, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	for {
		connp, err := listener.Accept()
		if err != nil {
			log.Printf("accept new client failed: %v", err)
			continue
		}

		go func() {
			client := tls.Server(connp, tlsCfg)
			handleHTTPClient(client)
		}()
	}

}

func httpForceHTTPS() {
	var err error
	defer log.Printf("http server exited. err: ", err)

	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := "https://" + r.Host + r.URL.Path

		if len(r.URL.RawQuery) > 0 {
			url += "?" + r.URL.RawQuery
		}

		http.Redirect(w, r, url, http.StatusSeeOther)
	})

	err = http.ListenAndServe(":80", m)
}

func httpServer() {
	listener, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Printf("accept new client failed: %v", err)
			continue
		}

		go handleHTTPClient(client)
	}
}

func handleHTTPClient(clientConn net.Conn) {
	if clientConn == nil {
		log.Printf("nil client")
		return
	}
	var err error

	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("accept new connection from %v", clientAddr)

	// client的链接可能单方面关闭
	// 为了避免另一方向的数据拷贝中断
	// 将他们放进两个goroutine并等待
	var wg sync.WaitGroup
	wg.Add(2)
	var chanConn = make(chan net.Conn, 1)

	// 将虚拟主机返回的数据传递给前端
	go func() {
		defer wg.Done()

		hostConn := <-chanConn
		if hostConn == nil {
			return
		}

		defer hostConn.(*net.TCPConn).CloseRead()

		defer func() {
			switch c := clientConn.(type) {
			case *net.TCPConn:
				c.CloseWrite()
			case *tls.Conn:
				c.CloseWrite()
			default:
			}
		}()

		_, err = io.Copy(clientConn, hostConn)
		hostAddr := hostConn.LocalAddr().String()
		log.Printf("copy from host %v to client %v. local %v err %v\n", hostAddr, clientAddr, hostConn.LocalAddr().String(), err)
	}()

	// 再将余下的数据传递虚拟主机
	go func() {
		defer wg.Done()
		// 通知从web服务器拷贝数据到客户端的协程退出
		defer close(chanConn)

		var hostConn net.Conn
		defer func() {
			if hostConn != nil {
				hostConn.(*net.TCPConn).CloseWrite()
			} else {
				log.Printf("host conn is nil")
			}
		}()

		defer func() {
			if c, ok := clientConn.(*net.TCPConn); ok {
				c.CloseRead()
			}
		}()

		// 存储解析出的虚拟主机名
		var host string
		reader := bufio.NewReader(clientConn)
		for {
			req, err := http.ReadRequest(reader)
			if err != nil {
				if err != io.EOF {
					log.Printf("read request, err: %v", err)
					return
				}

				break
			}

			const maxBufferSize = 4096
			buffer := bytes.NewBuffer(make([]byte, 0, maxBufferSize))
			if len(host) == 0 {
				host = req.Host
				log.Printf("host:   %v", host)

				// 找出虚拟主机地址
				var hostAddr string
				if _, ok := clientConn.(*net.TCPConn); ok {
					hostAddr, err = getVirtualHTTPHostAddr(host)
				} else {
					hostAddr, err = getVirtualHTTPSHostAddr(host)
				}
				if err != nil {
					log.Printf("err: unsupport host %v, err: %v", host, err)
					return
				}

				// 连接虚拟主机
				hostConn, err = net.Dial("tcp", hostAddr)
				if err != nil {
					log.Printf("err: connect to virtual host %v[%v] failed: %v", host, hostAddr, err)
					return
				}

				chanConn <- hostConn
			}

			// 插入客户端的真实ip信息到http头里
			clientIP, _, _ := net.SplitHostPort(clientAddr)
			req.Header.Add("X-Real-IP", clientIP)

			// 插入broker信息
			localIP, _, _ := net.SplitHostPort(hostConn.LocalAddr().String())
			prevXForwardFor := req.Header.Get("X-Forwarded-For")
			if len(prevXForwardFor) != 0 {
				prevXForwardFor = "," + prevXForwardFor
			}
			req.Header.Add("X-Forwarded-For", localIP+prevXForwardFor)

			req.Write(buffer)

			_, err = io.Copy(hostConn, buffer)
			log.Printf("copy from client %v to host %v. local %v err %v\n", clientAddr, "", hostConn.LocalAddr().String(), err)
		}

	}()

	wg.Wait()
}
