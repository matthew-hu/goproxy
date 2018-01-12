package proxy

import (
	`net`
	"log"
	"io"
	"regexp"
)

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

func (p *Proxy) handleConnUpstreamMode(client net.Conn) {
	defer func() {
		client.Close()
		leaving <- struct{}{}
	}()
	upConn, err := net.Dial("tcp", p.upstream)
	if err != nil {
		log.Println(err)
		return
	}
	defer upConn.Close()

	done := make(chan struct{})
	go func() {
		io.Copy(upConn, client)
		close(done)
	}()

	io.Copy(client, upConn)
	<- done
}
