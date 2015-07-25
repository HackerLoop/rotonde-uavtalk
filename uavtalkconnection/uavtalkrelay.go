package uavtalkconnection

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var listenSocket net.Conn

var relay struct {
	inChan  chan []byte
	outChan chan []byte

	connected bool
}{}

func initUAVTalkRelay(port int) (error) {
	r.inChan = make(chan []byte, 100)
	r.outChan = make(chan []byte, 100)

	r.ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	log.Infof("Relay listening %d", port)
	go func() {
		for {
			conn, err := r.ln.Accept()
			if err != nil {
				log.Warning(err)
				continue
			}
			r.connected = true
			log.Info("UTalkRelay connection started")

			errorChan := make(chan bool)

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()
				defer conn.Close()
				buffer := make([]byte, 1024)
				for {
					n, err := conn.Read(buffer)
					if err != nil {
						errorChan <- true
						log.Warning(err)
						return
					}
					r.inChan <- buffer[0:n]
				}
			}(wg)

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()
				defer conn.Close()
				for {
					select {
					case buffer := <-r.outChan:
						_, err := conn.Write(buffer)
						if err != nil {
							log.Warning(err)
							return
						}
					case <-errorChan:
						log.Info("errorChan triggered")
						return
					}
				}
			}(wg)

			wg.Wait()
			log.Info("UTalkRelay connection stopped")
			r.connected = false
		}
	}()

	return r, nil
}
