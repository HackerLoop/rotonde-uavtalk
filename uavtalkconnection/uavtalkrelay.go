package uavtalkconnection

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var listenSocket net.Conn

var relay = struct {
	inChan  chan []byte
	outChan chan []byte

	startStopChan chan bool
	stream        bool

	connected bool
}{}

func startRelayStream() {
	relay.startStopChan <- true
}

func stopRelayStream() {
	relay.startStopChan <- false
}

func initUAVTalkRelay(port int) error {
	relay.inChan = make(chan []byte, 100)
	relay.outChan = make(chan []byte, 100)

	relay.startStopChan = make(chan bool, 1)

	listenSocket, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	log.Infof("Relay listening %d", port)
	go func() {
		for {
			conn, err := listenSocket.Accept()
			if err != nil {
				log.Warning(err)
				continue
			}
			relay.connected = true
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

					select {
					case relay.stream = <-relay.startStopChan:
					default:
					}

					if relay.stream {
						relay.inChan <- buffer[0:n]
					}
				}
			}(wg)

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()
				defer conn.Close()
				for {
					select {
					case buffer := <-relay.outChan:
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
			relay.connected = false
		}
	}()

	return nil
}
