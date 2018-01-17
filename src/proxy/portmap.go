package proxy

import (
	"os"
	"log"
	"regexp"
	"bufio"
	"net"
	"io"
	"strings"
)

var portMapping = make(map[string]string, 10)

func init() {
	// load port mapping config
	f, err := os.Open("./config/portmap.txt")
	if err != nil {
		log.Printf("init: failed to load port mapping table: %v", err)
		return
	}
	defer f.Close()
	re := regexp.MustCompile(" +")
	input := bufio.NewScanner(f)
	for input.Scan() {
		if strings.HasPrefix(input.Text(), "#") {
			continue
		}
		result := re.Split(input.Text(), 5)
		if len(result) > 1 {
			portMapping[result[0]] = result[1]
		}
	}
}

func portMapDaemon() {
	for localPort, remoteTarget := range portMapping {
		go func(p, t string) {
			listener, err := net.Listen("tcp", ":"+p)
			if err != nil {
				log.Printf("failed to listen to port: %s, %v", p, err)
				return
			}
			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Printf("portMapDaemon: accept connection failed: will continue: %s, %v", p, err)
					continue
				}
				go func(client net.Conn) {
					defer client.Close()
					target, err := net.Dial("tcp", t)
					if err != nil {
						log.Printf("portMapDaemon: failed to connect to %s, %v", t, err)
						return
					}
					defer target.Close()
					//done := make(chan struct{})
					go func() {
						io.Copy(target, client)
						//close(done)
					}()
					io.Copy(client, target)
					//<- done
				}(conn)
			}
		}(localPort, remoteTarget)
	}
}