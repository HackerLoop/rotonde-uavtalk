package main

import (
	"fmt"
	"os"
	"time"

	"github.com/HackerLoop/rotonde-client.go"
	"github.com/HackerLoop/rotonde-uavtalk/uavtalk"
	log "github.com/Sirupsen/logrus"
	"github.com/vitaminwater/handlers.go"
)

type authPacketList []string

var authPackets = authPacketList{"SessionManaging", "FlightTelemetryStats", "GCSTelemetryStats"}

func (l authPacketList) contains(name string) bool {
	for _, s := range authPackets {
		if s == name {
			return true
		}
	}
	return false
}

/**
 *	UAVTalk protocol implementation
 */

func initAuthHandlers(root *handlers.HandlerManager, fcInChan chan uavtalk.Packet, client *client.Client) *handlers.HandlerManager {

	sessionManaging, err := uavtalk.AllDefinitions.GetDefinitionForName("SessionManaging")
	if err != nil {
		log.Fatal(err)
	}

	flightTelemetryStats, err := uavtalk.AllDefinitions.GetDefinitionForName("FlightTelemetryStats")
	if err != nil {
		log.Fatal(err)
	}

	handshakeReq := uavtalk.CreateGCSTelemetryStatsObjectPacket("HandshakeReq")
	fcInChan <- handshakeReq

	filter := func(i interface{}) (interface{}, bool) {
		p := i.(uavtalk.Packet)
		return i, authPackets.contains(p.Definition.Name)
	}

	connected := false
	disconnectedHandler := func(i interface{}) bool {
		p := i.(uavtalk.Packet)
		if p.Definition == flightTelemetryStats {
			if p.Data["Status"] == "Disconnected" {
				connected = false
				handshakeReq := uavtalk.CreateGCSTelemetryStatsObjectPacket("HandshakeReq")
				fcInChan <- handshakeReq
			} else if p.Data["Status"] == "HandshakeAck" {
				handshakeConnected := uavtalk.CreateGCSTelemetryStatsObjectPacket("Connected")
				fcInChan <- handshakeConnected
			}
		}
		return true
	}

	var sessionID uint16
	var currentObjectID uint8
	var numberOfObjects uint8

	/**
	 * TODO: This is getting messy, and the initial object retrieval is not done, but the
	 * process is still engaged on fc side, resulting in a 5 seconds pause of the stream,
	 * which should be avoidable.
	 */

	sessionHandler := func(i interface{}) bool {
		p := i.(uavtalk.Packet)
		if p.Definition == flightTelemetryStats {
			if !connected && p.Data["Status"] == "Connected" {
				connected = true
				sessionManagingReq := uavtalk.CreateSessionManagingRequest()
				fcInChan <- sessionManagingReq
			}
		} else if p.Definition == sessionManaging {
			if p.Cmd == uavtalk.ObjectCmd || p.Cmd == uavtalk.ObjectCmdWithAck {
				_numberOfObjects := p.Data["NumberOfObjects"].(uint8)
				if _numberOfObjects != 0 {
					numberOfObjects = _numberOfObjects
				}
				if p.Cmd == uavtalk.ObjectCmdWithAck {
					sessionManagingPacketAck := uavtalk.CreatePacketAck(p.Definition)
					fcInChan <- sessionManagingPacketAck

					objectID := p.Data["ObjectID"].(uint32)
					if objectID != 0 {
						definition, err := uavtalk.AllDefinitions.GetDefinitionForObjectID(objectID)
						if err != nil {
							log.Warning(err)
						} else {
							// TODO create rotonde definition
							log.Info("sending definition", definition.Name)
						}
					}

					if currentObjectID >= numberOfObjects {
						sessionID = sessionID
						log.Info("Available Definitions fetch done.")
						return true
					}

					if currentObjectID == 0 {
						sessionID = uint16(time.Now().Unix())
						log.Info("Creating session ", sessionID)
					}
					sessionManagingPacket := uavtalk.CreateSessionManagingPacket(sessionID, currentObjectID)
					currentObjectID++
					fcInChan <- sessionManagingPacket
				} else {
					_sessionID := p.Data["SessionID"].(uint16)
					// partial and bad session recovery
					log.Info("got sessionID ", _sessionID)
					if sessionID != 0 && sessionID == _sessionID {
						log.Info("Recovering ", sessionID)
						return true
					}
					sessionManagingPacket := uavtalk.CreateSessionManagingPacket(0, 0)
					fcInChan <- sessionManagingPacket
				}
			} else if p.Cmd == uavtalk.ObjectAck {
				log.Info("Received Ack for SessionManaging")
			} else if p.Cmd == uavtalk.ObjectNack {
				log.Info("Received Nack for SessionManaging")
			}
		}
		return true
	}

	auth := handlers.NewHandlerManager(root.NewOutChan(10), filter, handlers.Noop, handlers.Noop)
	auth.Attach(disconnectedHandler)
	auth.Attach(sessionHandler)
	return auth
}

func initStreamHandlers(root *handlers.HandlerManager, fcInChan chan uavtalk.Packet, client *client.Client) *handlers.HandlerManager {

	objectPersistenceDefinition, err := uavtalk.AllDefinitions.GetDefinitionForName("ObjectPersistence")
	if err != nil {
		log.Fatal(err)
	}

	filter := func(i interface{}) (interface{}, bool) {
		p := i.(uavtalk.Packet)
		return i, authPackets.contains(p.Definition.Name) == false
	}

	handler := func(i interface{}) bool {
		p := i.(uavtalk.Packet)
		if p.Cmd == uavtalk.ObjectCmdWithAck {
			fcInChan <- uavtalk.CreatePacketAck(p.Definition)
		} else if p.Cmd == uavtalk.ObjectAck {
			// send ObjectPersistence when received a Ack for object with Settings == true
			if p.Definition != objectPersistenceDefinition && p.Definition.Settings == true {
				fcInChan <- uavtalk.CreatePersistObject(p.Definition, p.InstanceID)
			}
		}
		return true
	}

	stream := handlers.NewHandlerManager(root.NewOutChan(10), filter, handlers.Noop, handlers.Noop)
	stream.Attach(handler)
	return stream
}

// main

func main() {
	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s common_directory/", os.Args[0]))
	}

	fcInChan := make(chan uavtalk.Packet, 100)
	fcOutChan := make(chan uavtalk.Packet, 100)

	client := client.NewClient("ws://127.0.0.1:4224")
	//rootIn := handlers.NewHandlerManager(chanCast(fcInChan), handlers.PassAll, handlers.Noop, handlers.Noop)

	uavtalk.LoadDefinitions(os.Args[1])
	go uavtalk.Start(fcInChan, fcOutChan)
	rootOut := handlers.NewHandlerManager(chanCast(fcOutChan), handlers.PassAll, handlers.Noop, handlers.Noop)
	initAuthHandlers(rootOut, fcInChan, client)
	initStreamHandlers(rootOut, fcInChan, client)

	select {}
}

// utils

func chanCast(inChan chan uavtalk.Packet) chan interface{} {
	outChan := make(chan interface{})

	go func() {
		defer close(outChan)
		for i := range inChan {
			outChan <- i
		}
	}()
	return outChan
}
