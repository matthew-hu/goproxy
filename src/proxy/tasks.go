package proxy

import (
	"os"
	"log"
	"bufio"
	"regexp"
	"strings"
	"html/template"
	"io/ioutil"
	"bytes"
	"fmt"
	"io"
)

var blackList = make(map[*regexp.Regexp]string, 10)

// load black list
func init() {
	f, err := os.Open("./config/blacklist.txt")
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	split := regexp.MustCompile(`\s+`)
	input := bufio.NewScanner(f)
	for input.Scan() {
		log.Println("parse blacklist: ", input.Text())
		fields := split.Split(input.Text(), 7)
		if len(fields) != 2 {
			// skip wrong format item
			continue
		}
		if strings.HasPrefix(fields[1], "*") {
			fields[1] = "." + fields[1]
		}
		blackList[regexp.MustCompile(fields[1])] = fields[0]
	}
	log.Println(blackList)
}

var blackListTemplate *template.Template

// load blacklist block template
func init() {
	f, err := os.Open("./pages/blacklistblock.html")
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	text, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println(err)
		return
	}
	blackListTemplate, err = template.New("blackListBlock").Parse(string(text))
	if err != nil {
		log.Println(err)
		return
	}
}

func scanTaskBlackListMatch(domain, url string) bool {
	for rgx, typ := range blackList {
		if typ == "domain" {
			if rgx.MatchString(domain) {
				return true
			}
		} else {
			if rgx.MatchString(url) {
				return true
			}
		}
	}
	return false
}

func takeActionBlackList(client io.Writer, url string) {
	writeResponseHeader(client, url, "blacklist")
}

func writeResponseHeader(client io.Writer, url string, tmpl string) {
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	if tmpl == "blacklist" {
		if blackListTemplate != nil {
			blackListTemplate.Execute(buf, url)
		}
	}

	header := []string {
		"HTTP/1.1 403 Forbidden\r\n",
		"Connection: Close\r\n",
		"Content-Type: text/html; charset=UTF-8\r\n",
		"Cache-Control: no-cache\r\n",
		fmt.Sprintf("Content-Length: %d\r\n\r\n", buf.Len()),
	}

	client.Write([]byte(strings.Join(header, "")))
	if buf.Len() > 0 {
		buf.WriteTo(client)
	}
}


