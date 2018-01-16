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
	authURL := strings.Split(client.LocalAddr().String(), ":")[0] + ":443"
	headers := "HTTP/1.1 307 Temporary Redirect\r\nConnection:Keep-Alive\r\n" +
		"Content-Length:0\r\nContent-Type:text/html; charset=UTF-8\r\n" +
			fmt.Sprintf("Location:https://%s/auth?forward=%s\r\n\r\n", authURL, url.QueryEscape(originalURL))
	_, err := io.WriteString(client, headers)
	return err

}

var users = map[string]string{"matthew": "111111"}

func authDaemon() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// send username password page
			log.Println("authDaemon: incoming auth request, method GET")
			if err := r.ParseForm(); err != nil {
				log.Printf("authDaemon: parse form err: %v", err)
			}

			if _, ok := r.Form["forward"]; ok {
				cookie := &http.Cookie{
					Name:   "OriginalURL",
					Value:    r.Form["forward"][0],
					Path:     "/",
					HttpOnly: false,
				}
				http.SetCookie(w, cookie)
				w.Header().Set("Connection", "keep-alive")
				w.Header().Set("Content-Length", string(len(authPage)))
				w.Header().Set("Content-Type", "text/html; charset=UTF-8")
				w.Write(authPage)
			}

			if _, ok := r.Form["username"]; ok {
				user, pass := r.Form["username"][0], r.Form["password"][0]
				if users[user] == pass {
					updateCache(strings.Split(r.RemoteAddr, ":")[0], user)
					log.Printf("authDaemon cache update success: %s: %s", user, pass)

					log.Println(r.Header)
					w.Header().Set("Connection", "close")
					w.Header().Set("Content-Length", "0")
					w.Header().Set("Content-Type", "text/html; charset=UTF-8")
					w.Header().Set("Location", strings.Split(r.Header["Cookie"][0], "=")[1])
					w.WriteHeader(http.StatusTemporaryRedirect)
				} else {
					w.Write(authPage)
				}
			}
		}
		//if r.Method == "POST" {
			// verify username and password
			// if fail send auth page again
			// else update ip user cache

		//}
	}

	http.HandleFunc("/auth", handler)
	log.Fatal(http.ListenAndServeTLS(":443", "server-cert.pem", "server-KEY.pem", nil))

}


