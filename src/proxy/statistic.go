package proxy

import (
	"log"
	"time"
)

func (p *Proxy) EnableStatistic() {
	p.enableStatistic = true
}

func (p *Proxy) connectionStatus() {
	var total, active, closed int64
	for {
		select {
		case <- incoming:
			total += 1
			active += 1
		case <- leaving:
			active -= 1
			closed += 1
		default:
			log.Printf("Total served connections: %d, active conntions: %d, closed connection: %d", total,
				active, closed)
			time.Sleep(5 * time.Second)
		}
	}
}
