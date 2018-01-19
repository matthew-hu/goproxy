package proxy

import (
	`net`
	"log"
	"io"
	"bufio"
	"strings"
	"io/ioutil"
)

func (p *Proxy) handleConnUpstreamMode(client net.Conn) {
	defer func() {
		client.Close()
		leaving <- struct{}{}
	}()

	done := make(chan struct{})

	upConn, err := net.Dial("tcp", p.upstream)
	if err != nil {
		log.Printf("handleConnUpstreamMode: falied to connect to upstream: %s, %v", p.upstream, err)
		return
	}
	defer upConn.Close()

	bufRD := bufio.NewReader(client)
	r, err := parseRequestHeader(bufRD)
	if err != nil && r == nil {
		log.Printf("parseRequestHeader: met err: %v", err)
		return
	}

	if r.proto == "https" {
		// 443 port authDaemon traffic
		if r.target == strings.Split(client.LocalAddr().String(), ":")[0]+":443" {
			authConn, err := net.Dial("tcp", r.target)
			if err != nil {
				log.Printf("handleConnUpstreamMode: failed to connect authDaemon: %v", err)
				return
			}
			defer authConn.Close()
			r.send(ioutil.Discard)
			io.WriteString(client, proxyString)
			go func(){
				io.Copy(authConn, client)
				done <- struct{}{}
			}()
			io.Copy(client, authConn)
			<- done
			return
		}
		log.Printf("handleConnUpstreamMode: incoming request: %v", *r)
		r.send(upConn)
		go func() {
			io.Copy(upConn, bufRD)
			done <- struct{}{}
		}()
	}

	go func() {
		io.Copy(client, upConn)
		done <- struct{}{}
	}()

	if r.proto == "http" {
		if p.enableAuth {
			if !queryCache(strings.Split(client.RemoteAddr().String(), ":")[0]) {
				if r.contentLength > 0 {
					log.Println("content-length", r.contentLength)
					discardRemainHeaders(r.rd, r.contentLength)
					r.contentLength = 0
				}
				log.Printf("redirect %s to auth", r.urlPath)
				authRedirect(client, r.proto + "://" + r.urlPath)
			}
		}

		if p.enableBlackList {
			log.Printf("checking blacklist match for first incoming request: %s", r.requestLine)
			if scanTaskBlackListMatch(r.domain, r.urlPath) {
				if r.contentLength > 0 {
					log.Println("content-length", r.contentLength)
					discardRemainHeaders(r.rd, r.contentLength)
					r.contentLength = 0
				}
				takeActionBlackList(client, r.proto + "://" + r.urlPath)
				return
			}
		}

		r.send(upConn)

		// handle more request
		for {
			r, err := parseRequestHeader(bufRD)
			if err != nil && r == nil {
				log.Printf("handleConnUpstreamMode: handle more request met err: %v", err)
				break
			}

			if p.enableBlackList {
				log.Printf("handleConnUpstreamMode: checking blacklist match for more request: %s", r.requestLine)
				if scanTaskBlackListMatch(r.domain, r.urlPath) {
					if r.contentLength > 0 {
						log.Println("content-length", r.contentLength)
						discardRemainHeaders(r.rd, r.contentLength)
						r.contentLength = 0
					}
					takeActionBlackList(client, r.proto + "://" + r.urlPath)
					break
				}
			}

			r.send(upConn)
		}
		// all requests have been sent
		done <- struct{}{}
	}

	// wait for both client and server finish data transfer
	<- done
	<- done
}


//func (p *Proxy) handleConnUpstreamMode(client net.Conn) {
//	defer func() {
//		client.Close()
//		leaving <- struct{}{}
//	}()
//	upConn, err := net.Dial("tcp", p.upstream)
//	if err != nil {
//		log.Printf("handleConnUpstreamMode: falied to connect to upstream: %s, %v", p.upstream, err)
//		return
//	}
//	defer upConn.Close()
//
//	done := make(chan struct{})
//	go func() {
//		io.Copy(upConn, client)
//		close(done)
//	}()
//
//	io.Copy(client, upConn)
//	<- done
//}