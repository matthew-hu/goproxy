package proxy

import (
	"regexp"
	"log"
	"net"
	"io"
)

var reverseHost = regexp.MustCompile(`.*:\d+`)


// to do: needs revise to get : scheme, host, port
func (p *Proxy) SetReverseHost(server string) {
	if !reverseHost.MatchString(server) {
		log.Fatal("invalid backend server for reverse proxy mode, should using: scheme://host:port format")
	}
	reverse, err := net.Dial("tcp", server)
	if err != nil {
		log.Fatalf("set reverse mode backend server failed: %v", err)
	}
	defer reverse.Close()
	p.reverse = server
}


// to do: needs to consider that the HOST field will be the proxy host, not the backend
// how does backend server dill with it
func (p *Proxy) handleConnReverseMode(client net.Conn) {
	defer func() {
		client.Close()
		leaving <- struct{}{}
	}()

	backend, err := net.Dial("tcp", p.reverse)
	if err != nil {
		log.Println(err)
		return
	}
	defer backend.Close()

	done := make(chan struct{})
	go func() {
		io.Copy(backend, client)
		close(done)
	}()

	io.Copy(client, backend)
}
