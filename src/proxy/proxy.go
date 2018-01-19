package proxy

import (
	`net`
	`log`
	"bufio"
	"strings"
	"io"
	"time"
	"io/ioutil"
	"regexp"
)


// used to counting handled connections
var incoming = make(chan struct{}, 1000)
var leaving = make(chan struct{}, 1000)

// used to send signal to other goroutines to stop there work
var stop = make(chan struct{})


type Proxy struct {
	port string
	upstream string
	enableBlackList bool
	enableStatistic bool
	enableAuth bool
	enablePortMap bool
}

func (p *Proxy) Start() {
	if p.port == "" {
		p.port = "8080"
	}

	listener, err := net.Listen("tcp", ":" + p.port)
	if err != nil {
		log.Fatal(err)
	}

	if p.enableStatistic {
		go p.connectionStatus()
	}

	if p.enableAuth {
		go cache()
		go authDaemon()
	}

	if p.enablePortMap {
		go portMapDaemon()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		select {
		case <- stop:
			break
		default:
		}

		incoming <- struct{}{}
		go p.handleConn(conn)
	}
}

func (p *Proxy) Stop() {
	close(stop)
}

func (p *Proxy) EnableBlackList() {
	p.enableBlackList = true
}

func (p *Proxy) EnableStatistic() {
	p.enableStatistic = true
}

func (p *Proxy) EnableAuth() {
	p.enableAuth = true
}

func (p *Proxy) EnablePortMap() {
	p.enablePortMap = true
}

var regProxy = regexp.MustCompile(`.*:\d+`)
func (p *Proxy) SetUpstreamProxy(prx string) {
	if !regProxy.MatchString(prx) {
		log.Fatalf("invalid upstram proxy %s, should using host:port format", prx)
	}
	upstream, err := net.Dial("tcp", prx)
	if err != nil {
		log.Fatalf("set upstream proxy mode failed: %v", err)
	}
	defer upstream.Close()
	p.upstream = prx
}

func (p *Proxy) handleConn(conn net.Conn) {
	if p.upstream != "" {
		// upstream proxy mode
		p.handleConnUpstreamMode(conn)
		return
	}

	p.handleConnPlain(conn)
}

var proxyString = "HTTP/1.1 200 Connection established\r\nProxy-agent: SimpleGoProxy\r\n\r\n"

func (p *Proxy) handleConnPlain(client net.Conn) {
	defer func() {
		client.Close()
		leaving <- struct{}{}
	}()

	connections := make(map[string]net.Conn)
	rd := bufio.NewReader(client)

	r, err := parseRequestHeader(rd)
	if err != nil && r == nil {
		log.Printf("handleConnPlain: parse first request fail: %v", err)
		return
	}

	log.Println("handle first coming request: ", r.requestLine)

	target, proto, domain, url := r.target, r.proto, r.domain, r.urlPath

	if p.enableAuth {
		// bypass access to auth daemon to avoid auth loop
		if proto == "http" && !queryCache(strings.Split(client.RemoteAddr().String(), ":")[0]) {
			if r.contentLength > 0 {
				log.Printf("need auth but content-length is %d, discard it first", r.contentLength)
				discardRemainHeaders(r.rd, r.contentLength)
				r.contentLength = 0
			}
			log.Printf("redirect %s to auth", url)
			authRedirect(client, proto + "://" + url)
		}
	}

	if p.enableBlackList {
		log.Printf("checking blacklist match for first incoming request: %s", r.requestLine)
		if scanTaskBlackListMatch(domain, url) {
			if r.contentLength > 0 {
				log.Printf("match blacklist but content-length is %d, discard it", r.contentLength)
				discardRemainHeaders(r.rd, r.contentLength)
				r.contentLength = 0
			}
			takeActionBlackList(client, proto + "://" + url)
			return
		}
	}

	server, err := net.Dial("tcp", target)
	if err != nil {
		log.Printf("fail to create connection to remote server: %v", err)
		return
	}
	connections[target] = server
	//defer server.Close()

	if proto == "https" {
		// send 200 connection established
		r.send(ioutil.Discard)
		io.WriteString(client, proxyString)
	} else {
		// send all request lines to server
		r.send(server)
	}

	// deliver server data to client
	handleEachServer := func(target string, s net.Conn) {
		defer s.Close()
		io.Copy(client, s)
		delete(connections, target)
	}

	go handleEachServer(target, server)

	// used by client to signal all requests have been done
	done := make(chan struct{})
	go p.handleMoreRequest(proto, rd, client, server, connections, done)

	<- done
	for len(connections) > 0 {
		time.Sleep(1 * time.Second)
	}
	log.Printf("close client connection: %v", client.RemoteAddr())
}

func (p *Proxy) handleMoreRequest(proto string, bufRdClient *bufio.Reader, currentClient net.Conn, currentServer net.Conn, connPool map[string]net.Conn, done chan<- struct{}) {
	defer func(){close(done)}()

	//this only works with http, as https is encrypt and can not see plain '\n'
	var currentConn = currentServer
	if proto == "http" {
		for {
			r, err := parseRequestHeader(bufRdClient)
			if err != nil && r == nil {
				log.Printf("handleMoreRequest parse more request fail, client: %v", currentClient.RemoteAddr())
				return
			}

			log.Println("handle more request: ", currentClient.RemoteAddr(), r.target, r.urlPath)
			target, domain, url := r.target, r.domain, r.urlPath

			if p.enableBlackList {
				log.Printf("handleMoreRequest: checking blacklist match for: %s\n", url)
				if scanTaskBlackListMatch(domain, url) {
					log.Printf("matched blacklist item: %s", url)
					if r.contentLength > 0 {
						discardRemainHeaders(bufRdClient, r.contentLength)
					}
					takeActionBlackList(currentClient, proto + "://" + url)
					return
				}
			}

			if _, ok := connPool[target]; !ok {
				server, err := net.Dial("tcp", target)
				if err != nil {
					log.Printf("handleMoreRequests, create new server conn: %v", err)
					return
				}
				log.Printf("handleMoreRequests, create new server conn: %s", target)
				connPool[target] = server
			}
			// go handleEachConn
			currentConn = connPool[target]

			r.send(currentConn)

			handleEachServer := func(target string, s net.Conn) {
				defer s.Close()
				io.Copy(currentClient, s)
				delete(connPool, target)
			}
			go handleEachServer(target, currentConn)
		}
		log.Println("leaving handleMoreRequest inside loop")

	} else {
		bufRdClient.WriteTo(currentServer)
	}

}

func discardRemainHeaders(rd *bufio.Reader, length int64) {
	n := int64(0)
	buf := make([]byte, 1024)
	for n < length {
		count, err := rd.Read(buf)
		if err != nil {
			return
		}
		n += int64(count)
	}
}
