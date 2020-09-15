package main

import (
	"broker/config"
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
)

var virtualHTTPHosts = map[string]config.VirtualHostConfig{}
var virtualHTTPSHosts = map[string]config.VirtualHostConfig{}

func getVirtualHTTPHostAddr(host string) (string, error) {
	host = strings.TrimSpace(host)
	v, ok := virtualHTTPHosts[host]
	if !ok {
		return "", fmt.Errorf("can not find %v", host)
	}

	return v.Host, nil
}

func getVirtualHTTPSHostAddr(host string) (string, error) {
	host = strings.TrimSpace(host)
	v, ok := virtualHTTPSHosts[host]
	if !ok {
		return "", fmt.Errorf("can not find %v", host)
	}

	return v.Host, nil
}
func main() {
	var cfgPath string
	var forceHTTPS bool
	flag.StringVar(&cfgPath, "config", "config.yaml", "config file path")
	flag.BoolVar(&forceHTTPS, "forcehttps", true, "use https only")
	flag.Parse()
	cfg, err := config.Read(cfgPath)
	if err != nil {
		log.Fatalf("read config file failed, err: %v", err)
	}

	for _, cfg := range cfg.HTTPHosts {
		virtualHTTPHosts[cfg.Domain] = cfg
	}
	for _, cfg := range cfg.HTTPSHosts {
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

	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("accept new connection from %v", clientAddr)

	// 存储解析出的虚拟主机名
	var host string

	// 要解析http头，就要读出数据，而这部分数据还必需发送到虚拟主机
	// 因此还需要将它们缓存起来
	const maxBufferSize = 4096
	buffer := bytes.NewBuffer(make([]byte, 0, maxBufferSize))

	reader := bufio.NewReader(io.TeeReader(clientConn, buffer))

	for {
		// 对于占用了很大缓存依旧没有分析到Host字段的请求
		// 直接认定为异常请求
		if buffer.Len() > maxBufferSize {
			log.Printf("err: invalid http header, the header is too large")
			return
		}

		data, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				//log.Printf(">>>>>>>>>> END <<<<<<")
				break
			}
		}
		line := string(data)
		if len(line) < 6 {
			continue
		}

		prefix := strings.ToLower(line[:5])
		if prefix != "host:" {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) != 2 && len(fields) != 3 {
			log.Printf("err: invalid host field in http header: %v", line)
			return
		}

		host = fields[1]

		// 顺便插入客户端的真实ip:port信息到http头里
		clientIP, _, _ := net.SplitHostPort(clientAddr)
		buffer.WriteString("X-Real-IP: " + clientIP + "\r\n")

		break
	}

	if len(host) == 0 {
		log.Printf("err: empty hostname")
		return
	}

	// 找出虚拟主机地址
	hostAddr, err := getVirtualHTTPHostAddr(host)
	if err != nil {
		log.Printf("err: unsupport host %v, err: %v", host, err)
		return
	}

	// 连接虚拟主机
	hostConn, err := net.Dial("tcp", hostAddr)
	if err != nil {
		log.Printf("err: connect to virtual host %v[%v] failed: %v", host, hostAddr, err)
		return
	}

	defer hostConn.Close()

	// 插入broker信息
	localIP, _, _ := net.SplitHostPort(hostConn.LocalAddr().String())
	buffer.WriteString("X-Forwarded-For: " + localIP + "\r\n")

	// client的链接可能单方面关闭
	// 为了避免另一方向的数据拷贝中断
	// 将他们放进两个goroutine并等待
	var wg sync.WaitGroup
	wg.Add(2)

	// 再将余下的数据传递虚拟主机
	go func() {
		defer wg.Done()
		defer hostConn.(*net.TCPConn).CloseWrite()
		defer func() {
			if c, ok := clientConn.(*net.TCPConn); ok {
				c.CloseRead()
			}
		}()

		_, err = io.Copy(hostConn, io.MultiReader(buffer, reader))
		log.Printf("copy from client %v to host %v. local %v err %v\n", clientAddr, hostAddr, hostConn.LocalAddr().String(), err)
	}()

	// 将虚拟主机返回的数据传递给前端
	go func() {
		defer wg.Done()
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
		log.Printf("copy from host %v to client %v. local %v err %v\n", hostAddr, clientAddr, hostConn.LocalAddr().String(), err)
	}()

	wg.Wait()
}
