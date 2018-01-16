package proxy

import (
	"net/http"
	"os"
	"log"
	"io/ioutil"
	"net"
	"fmt"
	"net/url"
	"io"
	"strings"
)


type qc struct {
	ip string
	ch chan string
}

type uc struct {
	ip string
	user string
}


// query ip user cache channel
var qChan = make(chan *qc, 100)

// update IP User Cache channel
var uChan = make(chan *uc, 100)

// auth page
var authPage []byte


func init() {
	// load auth page
	f, err := os.Open("./login.html")
	if err != nil {
		log.Fatalf("failed to load page login.html: %v", err)
	}
	defer f.Close()
	authPage, err = ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("failed to load page login.html: %v", err)
	}
}


func cache() {
	c := make(map[string]string, 100)
	for {
		select {
		case q := <- qChan:
			q.ch <- c[q.ip]
		case u := <- uChan:
			c[u.ip] = u.user
		case <- stop:
			break
		}
	}
}

func queryCache(ip string) bool {
	resp := make(chan string)
	qChan <- &qc{ip, resp}
	user := <- resp
	if user != "" {
		return true
	}
	log.Println("query ip get false")
	return false
}

func updateCache(ip, user string) {
	uChan <- &uc{ip, user}
}

func authRedirect(client net.Conn, originalURL string) error {
	authURL := strings.Split(client.LocalAddr().String(), ":")[0] + ":80"
	//authURL := "www.auth.com:80"
	headers := "HTTP/1.1 307 Temporary Redirect\r\nConnection:Keep-Alive\r\n" +
		"Content-Length:0\r\nContent-Type:text/html; charset=UTF-8\r\n" +
			fmt.Sprintf("Location:http://%s/auth?forward=%s\r\n\r\n", authURL, url.QueryEscape(originalURL))
	_, err := io.WriteString(client, headers)
	return err

}

func authDaemon() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// send username password page
			log.Println("xxxxxxxxxxxxxxxxxx----GET")
			w.Write(authPage)

		}
		if r.Method == "POST" {
			// verify username and password
			// if fail send auth page again
			// else update ip user cache
		}
	}

	http.HandleFunc("/auth", handler)
	log.Fatal(http.ListenAndServe(":80", nil))

}


