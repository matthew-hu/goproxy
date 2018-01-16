package proxy

import (
	"bytes"
	"bufio"
	"log"
	"regexp"
	"strings"
	"fmt"
	"strconv"
	"io"
)


type request struct {
	requestLine string
	method string
	proto string
	version string
	target string
	domain string
	url string
	path string
	header map[string][]string
	contentLength int
	rd *bufio.Reader
}

func (r *request) send(server io.Writer) {
	//log.Printf("%s %s %s\r\n", r.method, r.proto + "://" + r.url, r.version)
	//fmt.Fprintf(server, "%s %s %s\r\n", r.method, r.proto + "://" + r.url, r.version)
	log.Print(r.requestLine)
	fmt.Fprint(server, r.requestLine)
	for k, v := range r.header {
		for _, value := range v {
			//log.Printf("%s:%s\r\n", k, value)
			fmt.Fprintf(server, "%s:%s\r\n", k, value)
		}
	}
	fmt.Fprint(server, "\r\n")
	if r.contentLength >0 {
		log.Println("send: content-length: ", r.contentLength, r.requestLine)
		n := 0
		buf := make([]byte, 1024)
		for n < r.contentLength {
			count, err := r.rd.Read(buf)
			if err != nil {
				if count > 0 {
					server.Write(buf[:count])
				}
				break
			}

			if count > 0 {
				server.Write(buf[:count])
			}
			n += count
		}
		r.contentLength = 0
	}
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

func parseRequestHeader(rd *bufio.Reader) (r *request, err error) {
	var b []byte
	end := []byte("\r\n")
	for i := 0; bytes.Index(b, end) != 0; i++ {
		b, err = rd.ReadBytes('\n')
		if err != nil {
			log.Printf("parseRequestHeader i = %d: get error: %v", i, err)
			if len(b) > 0 {
				log.Printf("parseRequestHeader met error but have read some data: %s", string(b))
			}
			break
		}
		// at lease '\r\n'
		if len(b) < 2 {
			log.Println("parseRequestHeader: len(b) < 2", b)
			continue
		}

		if i == 0 {
			requestLine, method, proto, version, target, domain, url, path, err1 := parseFirstRequestLine(string(b))
			if err1 != nil {
				log.Println(err1)
				err = err1
				break
			}
			hd := make(map[string][]string)
			r = &request{requestLine, method, proto, version, target, domain, url, path, hd, 0, rd}
		} else {
			if k, v := parseRemainHeader(string(b)); k != "" && strings.ToLower(k) != "proxy-connection" {
				if strings.ToLower(k) == "content-length" {
					v = strings.TrimPrefix(v, " ")
					l, _ := strconv.ParseInt(v, 10, 32)
					r.contentLength = int(l)
				}
				r.header[k] = append(r.header[k], v)
			}
		}
	}
	return
}

func parseFirstRequestLine(line string) (requestLine, method, proto, version, target, domain, url, path string, err error) {
	fields := headerSplit.Split(line, 5)
	if len(fields) != 3 || !methods[strings.ToUpper(fields[0])] {
		err = fmt.Errorf("parseFirstRequestLine: not http(s) protocol: %s", line)
		for _, v := range fields {
			log.Println(v)
		}
		return
	}
	requestLine = line
	version = fields[2][:len(fields[2])-2] // strip ending "\r\n"
	if method = strings.ToUpper(fields[0]); method == "CONNECT" {
		target = fields[1]
		proto = "https"
		domain = strings.Split(target, ":")[0]
		url = domain
	} else {
		target = strings.Split(strings.Split(fields[1], "://")[1], "/")[0]
		proto = "http"
		url = strings.Split(fields[1], "://")[1]
		parts := strings.SplitN(url, "/", 2)
		if len(parts) == 1 || parts[1] == "" {
			path = "/"
		} else {
			path = "/" + parts[1]
		}
		if !strings.Contains(target, ":") {
			target += ":80"
		}
		domain = strings.Split(target, ":")[0]
	}
	return
}

func parseRemainHeader(line string) (key, value string) {
	// skip the http header and body split line "\r\n"
	if len(line) > 2 {
		parts := strings.SplitN(line[:len(line)-2], ":", 2)
		//log.Printf("parseRemainHeader: parts: %s --- %v\n", parts[0], parts)
		key = parts[0]
		if len(parts) > 1 {
			value = parts[1]
		}
	}
	return
}
