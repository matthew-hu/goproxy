package proxy

import (
	`net`
	`log`
	"bufio"
	"regexp"
	"strings"
	"fmt"
	"io"
	"bytes"
)


// used to counting handled connections
var incoming = make(chan struct{}, 1000)
var leaving = make(chan struct{}, 1000)


type Proxy struct {
	port string
	upstream string
	reverse string
	enableBlackList bool
	enableStatistic bool
}

func (p *Proxy) Start() {
	if p.port == "" {
		p.port = "8080"
	}

	listener, err := net.Listen("tcp", "localhost:" + p.port)
	if err != nil {
		log.Fatal(err)
	}

	if p.enableStatistic {
		go p.connectionStatus()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		incoming <- struct{}{}
		go p.handleConn(conn)
	}
}

func (p *Proxy) EnableBlackList() {
	p.enableBlackList = true
}

func (p *Proxy) handleConn(conn net.Conn) {
	if p.reverse != "" {
		// reverse proxy mode
		return
	}

	if p.upstream != "" {
		go p.handleConnUpstreamMode(conn)
		return
	}

	go p.handleConnPlain(conn)
}

var proxyString = "HTTP/1.1 200 Connection established\r\nProxy-agent: SimpleProxy\r\n\r\n"

func (p *Proxy) handleConnPlain(client net.Conn) {
	defer func() {
		client.Close()
		leaving <- struct{}{}
	}()

	// read the first line of the client request to determine the destination server to connect
	readerClient := bufio.NewReader(client)
	firstLine, err := readerClient.ReadString('\n')
	if err != nil {
		log.Printf("read first line, get error: %v", err)
		return
	}

	target, proto, domain, url, err := parseServer(firstLine)
	if err != nil {
		log.Println(err)
		return
	}

	if p.enableBlackList {
		if scanTaskBlackListMatch(domain, url) {
			discardRemainHeaders(readerClient)
			takeActionBlackList(client, proto + "://" + url)
			return
		}
	}

	server, err := createServerConn(target)
	if err != nil {
		log.Println(err)
		return
	}
	defer server.Close()

	if proto == "https" {
		// read the remain headers of CONNECT request
		//for {
		//	s, _ := readerClient.ReadString('\n')
		//	if index := strings.Index(s, "\r\n"); index == 0 {
		//		break
		//	}
		//}
		discardRemainHeaders(readerClient)
		// send 200 connection established
		io.WriteString(client, proxyString)
	} else {
		// send all request lines to server
		io.WriteString(server, firstLine)
		for {
			s, _ := readerClient.ReadString('\n')
			io.WriteString(server, s)
			if index := strings.Index(s, "\r\n"); index == 0 {
				break
			}
		}
	}

	// used by client to signal all requests have been done
	done := make(chan struct{})
	go handleMoreRequest(readerClient, server, done)

	// deliver server data to client
	io.Copy(client, server)
	<- done

}

// split client request line, such as: CONNECT www.baidu.com HTTP/1.1
var headerSplit = regexp.MustCompile(` +`)
var methods = map[string]bool{
	"GET": true,
	"POST": true,
	"PUT": true,
	"DELETE": true,
	"HEAD": true,
	"OPTIONS": true,
	"TRACE": true,
	"CONNECT": true,
}

// get target host, proto, domain, url from client request
func parseServer(header string) (target, proto, domain, url string, err error) {
	fields := headerSplit.Split(header, 5)
	if len(fields) != 3 || !methods[strings.ToUpper(fields[0])] {
		err = fmt.Errorf("parseServer: not http(s) protocol: %s", header)
		return
	}

	if strings.ToUpper(fields[0]) == "CONNECT" {
		target = fields[1]
		proto = "https"
		domain = strings.Split(target, ":")[0]
		url = domain
	} else {
		target = strings.Split(strings.Split(fields[1], "://")[1], "/")[0]
		proto = "http"
		url = strings.Split(fields[1], "://")[1]
		if !strings.Contains(target, ":") {
			target += ":80"
		}
		domain = strings.Split(target, ":")[0]
	}
	return
}

func createServerConn(target string) (server net.Conn, err error) {
	server, err = net.Dial("tcp", target)
	return
}

func handleMoreRequest(rdClient io.Reader, wrServer io.Writer, done chan<- struct{}) {
	buf := make([]byte, 4096)
	reqIdentify := []byte(" HTTP/1.")
	for {
		n, err := rdClient.Read(buf)
		if err != nil {
			log.Printf("read more request from client meets error: %v", err)
			if n > 0 {
				// send last bytes
				wrServer.Write(buf[:n])
				log.Printf("last bytes of client request have been sent: %s", string(buf[:n]))
			}
			break
		}
		if bytes.Index(buf[:n], reqIdentify) != -1 {
			log.Printf("client issues another request: \n%s", string(buf[:n]))
		}
		wrServer.Write(buf[:n])
	}
	close(done)
}

func discardRemainHeaders(rd io.Reader) {
	reader := rd.(*bufio.Reader)
	for {
		s, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if strings.Index(s, "\r\n") == 0 {
			break
		}
	}
}