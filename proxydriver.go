package main

import (
	"log"
	"os"
	"proxy"
	//"runtime/pprof"
)

func main() {

	//f, err := os. Create("cpu.prof")
	//if err != nil {
	//	return
	//}
	//pprof.StartCPUProfile(f)
	//defer pprof.StopCPUProfile()

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	prx := new(proxy.Proxy)
	prx.EnableStatistic()
	prx.EnableBlackList()
	//prx.SetUpstreamProxy("10.202.241.54:8080")
	prx.EnableAuth()

	prx.Start()
	//time.Sleep(100 * time.Second)
	//prx.Stop()
}
