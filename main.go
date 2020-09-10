package main

import (
	"broker/config"
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

var virtualHTTPHostAddr = map[string]string{}
var virtualHTTPSHostAddr = map[string]string{}

func getVirtualHTTPHostAddr(host string) (string, error) {
	host = strings.TrimSpace(host)
	v, ok := virtualHTTPHostAddr[host]
	if !ok {
		return "", fmt.Errorf("can not find %v", host)
	}

	return v, nil
}

func getVirtualHTTPSHostAddr(host string) (string, error) {
	host = strings.TrimSpace(host)
	v, ok := virtualHTTPSHostAddr[host]
	if !ok {
		return "", fmt.Errorf("can not find %v", host)
	}

	return v, nil
}
func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.yaml", "config file path")
	flag.Parse()
	cfg, err := config.Read(cfgPath)
	if err != nil {
		log.Fatalf("read config file failed, err: %v", err)
	}

	for _, h := range cfg.HTTPHosts {
		virtualHTTPHostAddr[h.Domain] = h.Host
	}
	for _, h := range cfg.HTTPSHosts {
		virtualHTTPSHostAddr[h.Domain] = h.Host
	}

	go httpsHandler()
	httpHandler()
}

func httpsHandler() {
	listener, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Printf("accept new client failed: %v", err)
			continue
		}

		go handleHTTPSClient(client)
	}
}

type readOnlyConn struct {
	reader io.Reader
}

func (c readOnlyConn) Read(p []byte) (int, error)         { return c.reader.Read(p) }
func (c readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (c readOnlyConn) Close() error                       { return nil }
func (c readOnlyConn) LocalAddr() net.Addr                { return nil }
func (c readOnlyConn) RemoteAddr() net.Addr               { return nil }
func (c readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (c readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (c readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

func getSNI(reader io.Reader) (string, io.Reader, error) {
	var err error

	buffer := new(bytes.Buffer)
	r := io.TeeReader(reader, buffer)

	var hello *tls.ClientHelloInfo
	err = tls.Server(readOnlyConn{reader: r}, &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = new(tls.ClientHelloInfo)
			*hello = *argHello
			return nil, nil
		},
	}).Handshake()

	if hello == nil {
		return "", nil, err
	}
	serverName := hello.ServerName

	return serverName, io.MultiReader(buffer, reader), nil
}

func handleHTTPSClient(clientConn net.Conn) {
	if clientConn == nil {
		log.Printf("nil client")
		return
	}

	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("accept new https connection from %v", clientAddr)

	host, reader, err := getSNI(clientConn)
	if err != nil {
		log.Printf("err: parse SNI failed: %v", err)
		return
	}

	// 连接虚拟主机
	hostAddr, err := getVirtualHTTPSHostAddr(host)
	if err != nil {
		log.Printf("err: unsupport host %v, err: %v", host, err)
		return
	}

	hostConn, err := net.Dial("tcp", hostAddr)
	if err != nil {
		log.Printf("err: connect to virtual host %v[%v] failed: %v", host, hostAddr, err)
		return
	}

	defer hostConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err = io.Copy(hostConn, reader)
		log.Printf("copy from client %v to host %v. err %v", clientAddr, hostAddr, err)
	}()

	go func() {
		defer wg.Done()
		_, err = io.Copy(clientConn, hostConn)
		log.Printf("copy from host %v to client %v. err %v", hostAddr, clientAddr, err)
	}()

	wg.Wait()
}

func httpHandler() {
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

	// 开始解析Host
	scanner := bufio.NewScanner(clientConn)

	// 限制扫描的时候分配的缓冲区大小
	// 如果不限制，则会使用scanner默认的缓冲区大小：MaxScanTokenSize = 64 * 1024
	scannerBuffer := make([]byte, 256)
	scanner.Buffer(scannerBuffer, len(scannerBuffer))

	for scanner.Scan() {
		if buffer.Len() > maxBufferSize {
			log.Printf("err: invalid http header, the header is too large")
			return
		}

		line := scanner.Text()
		buffer.WriteString(line + "\r\n")

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

	// client的链接可能单方面关闭
	// 为了避免另一方向的数据拷贝中断
	// 将他们放进两个goroutine并等待
	var wg sync.WaitGroup
	wg.Add(2)

	// 再将余下的数据传递虚拟主机
	go func() {
		defer wg.Done()
		_, err = io.Copy(hostConn, io.MultiReader(buffer, clientConn))
		log.Printf("copy from client %v to host %v. err %v", clientAddr, hostAddr, err)
	}()

	// 将虚拟主机返回的数据传递给前端
	go func() {
		defer wg.Done()
		_, err = io.Copy(clientConn, hostConn)
		log.Printf("copy from host %v to client %v. err %v", hostAddr, clientAddr, err)
	}()

	wg.Wait()
}
