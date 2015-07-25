package uavtalkconnection

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
)

type relay struct {
	inChan  chan []byte
	outChan chan []byte

	connected bool
}

func newUAVTalkRelayChan(port int) (*relay, error) {
	r := &relay{}
	r.inChan = make(chan []byte, 100)
	r.outChan = make(chan []byte, 100)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	log.Infof("Relay listening %d", port)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Warning(err)
				continue
			}
			r.connected = true

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func(wg sync.WaitGroup) {
				defer wg.Done()
				buffer := make([]byte, 1024)
				for {
					n, err := conn.Read(buffer)
					if err != nil {
						log.Warning(err)
						return
					}
					r.inChan <- buffer[0:n]
				}
			}(wg)

			go func(wg sync.WaitGroup) {
				defer wg.Done()
				for {
					buffer := <-r.outChan

					_, err := conn.Write(buffer)
					if err != nil {
						log.Warning(err)
						return
					}
				}
			}(wg)

			wg.Wait()
			r.connected = false
		}
	}()

	return r, nil
}
