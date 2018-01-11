package main

import (
	"log"
	"os"
	"proxy"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	prx := new(proxy.Proxy)
	prx.Start()
}
