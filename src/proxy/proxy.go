package proxy

import (
	`net`
	`log`
	"os"
	"bufio"
	"regexp"
	"strings"
	"fmt"
	"io"
	"time"
	"bytes"
)

var blackList = make(map[string]bool, 10)
// used to counting handled connections
var incoming = make(chan struct{}, 1000)
var leaving = make(chan struct{}, 1000)

// load black list
func init() {
	f, err := os.Open("./blacklist.txt")
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	input := bufio.NewScanner(f)
	for input.Scan() {
		log.Println(input.Text())
		blackList[input.Text()] = true
	}
}

type Proxy struct {
	port string
	upstream string
	enableBlackList bool
}

func (p *Proxy) Start() {
	if p.port == "" {
		p.port = "8080"
	}
	listener, err := net.Listen("tcp", "localhost:" + p.port)
	if err != nil {
		log.Fatal(err)
	}
	go connectionStatus()
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

func connectionStatus() {
	var total, active, closed int64
	for {
		select {
		case <- incoming:
			total += 1
			active += 1
		case <- leaving:
			active -= 1
			closed += 1
		default:
			log.Printf("Total served connections: %d, active conntions: %d, closed connection: %d", total,
				active, closed)
			time.Sleep(5 * time.Second)
		}
	}
}

func (p *Proxy) handleConn(conn net.Conn) {
	if p.upstream != "" {
		go handleConnUpstreamMode(conn)
	} else {
		go p.handleConnPlain(conn)
	}
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

	server, proto, err := createServerConn(firstLine)
	if err != nil {
		return
	}
	defer server.Close()

	if proto == "https" {
		// read the remain headers of CONNECT request
		for {
			s, _ := readerClient.ReadString('\n')
			if index := strings.Index(s, "\r\n"); index == 0 {
				break
			}
		}
		io.WriteString(client, proxyString)
	} else {
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

func parseServer(header string) (target, proto string, err error) {
	fields := headerSplit.Split(header, 5)
	if len(fields) != 3 || !methods[strings.ToUpper(fields[0])] {
		err = fmt.Errorf("parseServer: not http(s) protocol: %s", header)
		return
	}

	if strings.ToUpper(fields[0]) == "CONNECT" {
		return fields[1], "https", nil
	} else {
		target = strings.Split(strings.Split(fields[1], "://")[1], "/")[0]
		if !strings.Contains(target, ":") {
			target += ":80"
		}
		proto = "http"
	}
	return
}

func createServerConn(statusLine string) (server net.Conn, proto string, err error) {
	target, proto, err := parseServer(statusLine)
	if err != nil {
		log.Println(err)
		return nil, proto, err
	}
	//log.Println(target, proto)
	server, err = net.Dial("tcp", target)
	if err != nil {
		log.Println(err)
		return nil, proto, err
	}
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