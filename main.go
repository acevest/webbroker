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
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
	"webbroker/config"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "c", "config.yaml", "config file path")
	flag.Parse()
	err := config.Read(cfgPath)
	if err != nil {
		log.Fatalf("read config file failed, err: %v", err)
	}

	httpsServer()
}

type xHost struct {
	target *url.URL
	prefix string
	host   string
	port   string
}
type proxy struct {
	targets map[string]xHost
}

func newProxy() *proxy {
	return &proxy{
		targets: make(map[string]xHost),
	}
}

func (p *proxy) addConfig() {
	for _, c := range config.Conf.HTTPSServers {
		log.Printf(">>>>> http: %v\n", c)
		log.Printf(">>>>>> domain:%v host %v port %v prefix:%v len: %v", c.Domain, c.Host, c.Port, c.Prefix, len(c.Prefix))

		p.addTarget(c.Domain, c.Prefix, c.Host, c.Port)
	}
}

func (p *proxy) addTarget(domain, prefix, host, port string) error {

	key := fmt.Sprintf("%v%v", domain, prefix)
	if len(prefix) == 0 {
		key = fmt.Sprintf("%v%v", domain, "/")
	}
	targetURL, _ := url.Parse(fmt.Sprintf("http://%v:%v", host, port))

	log.Printf("add key %v target %v", key, targetURL)
	p.targets[key] = xHost{
		target: targetURL,
		prefix: prefix,
		host:   host,
		port:   port,
	}
	return nil
}

// func (p *proxy) addTarget(path, target string) error {
// 	targetURL, err := url.Parse(target)
// 	if err != nil {
// 		return err
// 	}
// 	p.targets[path] = targetURL
// 	return nil
// }

func commonPrefixLength(s1, s2 string) int {
	length := 0
	for i := 0; i < len(s1) && i < len(s2); i++ {
		if s1[i] == s2[i] {
			length++
		} else {
			break
		}
	}
	return length
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// var target *url.URL

	// if strings.HasPrefix(r.URL.Path, "/bbb") {
	// 	target = p.targets["/bbb"]
	// } else {
	// 	target = p.targets["default"]
	// }

	log.Printf("req host %v url %v ", r.Host, r.URL.Path)
	url := r.Host + r.URL.Path
	maxPrefixLen := 0
	var target xHost
	var found bool
	for k, v := range p.targets {
		log.Printf("DD: %v", k)
		prefixLen := commonPrefixLength(url, k)
		log.Printf("%v %v prefix len %v", url, k, prefixLen)
		if prefixLen == 0 {
			continue
		}

		// if prefixLen == maxPrefixLen {
		// 	log.Fatalf("prefix len == max prefix len %v %v", url, k)
		// }

		if prefixLen > maxPrefixLen {
			maxPrefixLen = prefixLen
			target = v
			found = true
			log.Printf("req host %v url %v find %v", r.Host, r.URL.Path, v)
		} else {
			log.Printf("fucked")
		}
	}

	if !found {
		log.Printf("dasfsadfasdfs")
		return
	}

	log.Printf("result %v", target.target)

	log.Printf("xxx %v", r.URL.Path)
	r.URL.Path = strings.TrimLeft(r.URL.Path, target.prefix)
	if !strings.HasPrefix(r.URL.Path, "/") {
		r.URL.Path = "/" + r.URL.Path
	}
	log.Printf("xxx %v", r.URL.Path)
	r.URL.Host = target.target.Host
	r.URL.Scheme = target.target.Scheme
	r.Host = target.target.Host
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Error making request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func httpsServer() {
	var err error
	proxy := newProxy()

	// err = proxy.addTarget("default", "http://localhost:20000")
	// if err != nil {
	// 	log.Println("Error adding target:", err)
	// 	return
	// }
	proxy.addConfig()

	certificates := make(map[string]tls.Certificate)
	for _, cfg := range config.GetAllHTTPSServer() {
		certPath := path.Join(config.CertsPath, cfg.Domain+"_bundle.crt")
		keyPath := path.Join(config.CertsPath, cfg.Domain+".key")
		certificates[cfg.Domain], err = tls.LoadX509KeyPair(certPath, keyPath)
		log.Printf("load %v %v", certPath, keyPath)
		if err != nil {
			log.Fatalf("load certificates failed %v", err)
		}

	}

	server := &http.Server{
		Addr:    ":443",
		Handler: proxy,
		TLSConfig: &tls.Config{
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, ok := certificates[hello.ServerName]
				if !ok {
					return nil, fmt.Errorf("no certificate for domain: %s", hello.ServerName)
				}
				return &cert, nil
			},
		},
	}

	log.Println("Starting HTTPS proxy server on port 443")

	err = server.ListenAndServeTLS("", "")
	if err != nil {
		log.Println("Error starting server:", err)
	}
}

func httpPortServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	log.Fatal(http.ListenAndServe(":80", nil))
}

func old_httpsServer() {
	tlsCfg := &tls.Config{}
	for _, cfg := range config.GetAllHTTPSServer() {
		certPath := path.Join(config.CertsPath, cfg.Domain+"_bundle.crt")
		keyPath := path.Join(config.CertsPath, cfg.Domain+".key")
		log.Printf("%v %v\n", certPath, keyPath)
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			log.Printf("ERR: %v\n", err)

			certPath = path.Join(config.CertsPath, "1_"+cfg.Domain+"_bundle.crt")
			keyPath = path.Join(config.CertsPath, "2_"+cfg.Domain+".key")
			log.Printf("%v %v\n", certPath, keyPath)
			cert, err = tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				log.Fatal(err)
			}
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
	defer log.Printf("http server exited. err: %v", err)

	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := "https://" + r.Host + r.URL.Path

		if len(r.URL.RawQuery) > 0 {
			url += "?" + r.URL.RawQuery
		}

		http.Redirect(w, r, url, http.StatusSeeOther)
	})

	err = http.ListenAndServe(":80", m)
	log.Printf("http force https, err: %v", err)
}

func httpServer(addr string, secure bool) {
	log.Printf("http server listen at: %v", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	for {
		client, err := listener.Accept()
		client = &SecureConn{client, secure}

		// a := SecureConn{client}
		// log.Printf("a %v", a.LocalAddr())
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

		defer hostConn.(Closer).CloseRead()

		defer func() {
			switch c := clientConn.(type) {
			case Closer:
				c.CloseWrite()
			case *tls.Conn:
				c.CloseWrite()
			default:
			}
		}()

		_, err = io.Copy(clientConn, hostConn)
		//hostAddr := hostConn.LocalAddr().String()
		log.Printf("copy from server %v to client %v. local %v err %v\n", hostConn.RemoteAddr(), clientAddr, hostConn.LocalAddr().String(), err)
	}()

	// 再将余下的数据传递虚拟主机
	go func() {
		defer wg.Done()
		// 通知从web服务器拷贝数据到客户端的协程退出
		defer close(chanConn)

		var hostConn net.Conn
		defer func() {
			if hostConn != nil {
				hostConn.(Closer).CloseWrite()
			} else {
				log.Printf("host conn is nil")
			}
		}()

		defer func() {
			if c, ok := clientConn.(Closer); ok {
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
			fmt.Println(req)

			const maxBufferSize = 4096
			buffer := bytes.NewBuffer(make([]byte, 0, maxBufferSize))
			if len(host) == 0 {
				host = req.Host
				path := req.URL.Path

				// 找出虚拟主机地址
				var hostAddr string
				var secureMode bool
				var cfg *config.VirtualServerConfig
				if _, ok := clientConn.(*tls.Conn); ok {
					cfg, err = config.GetVirtualHTTPSServerAddr(host, path)
				} else {
					cfg, err = config.GetVirtualHTTPSServerAddr(host, path)
				}
				if err != nil {
					log.Printf("err: unsupport host %v, err: %v", host, err)
					return
				}

				hostAddr = cfg.Addr()
				secureMode = cfg.SecureMode

				log.Printf("host:   %v", host)
				log.Printf("FUKC : %v", path)
				if len(cfg.Prefix) > 0 {
					req.URL.Path = strings.Replace(path, cfg.Prefix, "/", 1)
				}
				log.Printf("FUKC : %v", req.URL.Path)

				// 连接虚拟主机
				hostConn, err = net.Dial("tcp", hostAddr)
				if err != nil {
					log.Printf("err: connect to virtual host %v[%v] failed: %v", host, hostAddr, err)
					return
				}

				hostConn = &SecureConn{hostConn, secureMode}

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
			log.Printf("copy from client %v to server %v. local %v err %v\n", clientAddr, hostConn.RemoteAddr().String(), hostConn.LocalAddr().String(), err)
			if req.Header.Get("Upgrade") == "websocket" {
				log.Printf("begin to copy websocket data from client %v to server %v.", clientAddr, hostConn.RemoteAddr().String())
				_, err = io.Copy(hostConn, reader)
				log.Printf("copy websocket from client %v to server %v. local %v err %v\n", clientAddr, hostConn.RemoteAddr().String(), hostConn.LocalAddr().String(), err)
				break
			}
		}

	}()

	wg.Wait()
}
