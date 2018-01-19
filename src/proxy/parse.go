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
	"net/url"
)


type request struct {
	requestLine string
	proto string
	target string
	domain string
	urlPath string
	header map[string][]string
	contentLength int64
	chunk bool
	rd *bufio.Reader
}

func (r *request) send(server io.Writer) {
	//log.Print(r.requestLine)
	fmt.Fprint(server, r.requestLine)
	for k, v := range r.header {
		for _, value := range v {
			//log.Printf("%s:%s\r\n", k, value)
			fmt.Fprintf(server, "%s:%s\r\n", k, value)
		}
	}
	fmt.Fprint(server, "\r\n")

	if r.contentLength > 0 && !r.chunk {
		log.Println("send: content-length: ", r.contentLength, r.requestLine)
		n := int64(0)
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
			n += int64(count)
		}
		r.contentLength = 0
	}

	if r.chunk { // The Content-Length header is omitted in this case
		log.Printf("send: client using transfer-encoding: chunked, handle it specially: %s", r.domain)
		var b []byte
		var chunkEnd = []byte("\r\n")
		var err error
		for bytes.Index(b, chunkEnd) != 0 {
			b, err = r.rd.ReadBytes('\n')
			if err != nil {
				log.Printf("send: chunked: %v", err)
				if len(b) > 0 {
					server.Write(b)
				}
				break
			}
			server.Write(b)
		}
		log.Println("send: chunked data has been sent")
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
		// protect the scenario that user send so long data that is not http(s) with no \r\n found when rd.ReadBytes('\n')
		if i == 0 {
			// at least 'CONNECT x', 9 characters
			data, _ := rd.Peek(9)
			if _, ok := methods[strings.Split(strings.ToUpper(string(data)), " ")[0]]; !ok {
				err = fmt.Errorf("peek first request line for 9 bytes to check proto quickly failed: %s", string(data))
				return
			}
		}
		b, err = rd.ReadBytes('\n')
		if err != nil {
			log.Printf("parseRequestHeader: request line %d: get error: %v", i+1, err)
			if len(b) > 0 {
				log.Printf("parseRequestHeader met error but have read some data: %s", string(b))
			}
			break
		}
		// at lease '\r\n'
		if len(b) < 2 {
			log.Printf("parseRequestHeader: request line %d, to short: %v, will continue", len(b), b)
			continue
		}

		if i == 0 {
			requestLine, proto, target, domain, urlPath, err1 := parseFirstRequestLine(string(b))
			if err1 != nil {
				log.Printf("parseRequestHeader: %v", err1)
				err = err1
				break
			}
			hd := make(map[string][]string)
			r = &request{
				requestLine: requestLine,
				proto: proto,
				target: target,
				domain: domain,
				urlPath: urlPath,
				header: hd,
				rd: rd,
			}

		} else {
			if k, v := parseRemainHeader(string(b)); k != "" && strings.ToLower(k) != "proxy-connection" {
				key := strings.ToLower(k)
				if key == "content-length" {
					v = strings.TrimPrefix(v, " ")
					cl, _ := strconv.ParseInt(v, 10, 64)
					r.contentLength = cl
				}

				if key == "transfer-encoding" && strings.Contains(v,"chunked") {
					r.chunk = true
				}
				r.header[k] = append(r.header[k], v)
			}
		}
	}
	return
}

func parseFirstRequestLine(line string) (requestLine, proto, target, domain, urlPath string, err error) {
	fields := headerSplit.Split(line, 5)
	if len(fields) != 3 || !methods[strings.ToUpper(fields[0])] {
		err = fmt.Errorf("parseFirstRequestLine: not http(s) protocol: %s", line)
		return
	}
	requestLine = line
	if method := strings.ToUpper(fields[0]); method == "CONNECT" {
		target = fields[1]  //host:port format
		proto = "https"
		domain = strings.Split(target, ":")[0]
		urlPath = domain
	} else {
		rawurl := fields[1]
		u, errURL := url.Parse(rawurl)
		if errURL != nil {
			err = fmt.Errorf("parseFirstRequestLine: %v", errURL)  // URL parse failed
			return
		}

		target = u.Host
		if u.Port() == "" {
			target += ":80"
		}

		proto = u.Scheme
		domain = u.Hostname()
		if len(strings.SplitN(rawurl, "://", 2)) > 1 {
			urlPath = strings.SplitN(rawurl, "://", 2)[1]
		} else if len(strings.SplitN(rawurl, "%3A%2F%2F", 2)) > 1 { // deal with http%3A%2F%2F -> http://
			urlPath = strings.SplitN(rawurl, "%3A%2F%2F", 2)[1]
		}

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
