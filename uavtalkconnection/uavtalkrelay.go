package uavtalkconnection

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/openskybot/skybot-router/utils"
)

var listenSocket net.Conn

var Relay = struct {
	InChan  chan []byte
	OutChan chan []byte

	stream bool

	Connected bool
}{}

func StartRelayStream() {
	log.Info("Starting relay stream")
	Relay.stream = true
	log.Info("Relay stream started")
}

func StopRelayStream() {
	log.Info("Stopping relay stream")
	Relay.stream = false
	log.Info("Relay stream stopped")
}

func InitUAVTalkRelay(port int) error {
	Relay.InChan = make(chan []byte, 100)
	Relay.OutChan = make(chan []byte, 100)

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
			Relay.Connected = true
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

					log.Info("from tcp connection:")
					utils.PrintHex(buffer, n)
					packet, err := newPacketFromBinary(buffer[0:n])
					if err != nil {
						log.Warning(err)
					} else {
						log.Info(packet.definition.Name, packet)
					}

					if Relay.stream {
						Relay.InChan <- buffer[0:n]
					}
				}
			}(wg)

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()
				defer conn.Close()
				for {
					select {
					case buffer := <-Relay.OutChan:
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
			Relay.Connected = false
		}
	}()

	return nil
}
