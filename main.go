package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/HackerLoop/rotonde-client.go"
	"github.com/HackerLoop/rotonde-uavtalk/uavtalk"
	"github.com/HackerLoop/rotonde/shared"
	log "github.com/Sirupsen/logrus"
	"github.com/vitaminwater/handlers.go"
)

const SESSION_PAUSE = 5

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
	 * process is still engaged on fc side, resulting in a SESSION_PAUSE seconds pause of the stream,
	 * which should be avoidable.
	 */

	var start time.Time
	var activeDefinitions []*uavtalk.Definition
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
							activeDefinitions = append(activeDefinitions, definition)
						}
					}

					if currentObjectID >= numberOfObjects {
						sessionID = sessionID
						log.Info("Available Definitions fetch done.")
						spent := time.Now().Sub(start).Seconds()
						go func() {
							meta := map[string]interface{}{
								"modes": float64(0), "periodFlight": float64(0), "periodGCS": float64(0), "periodLog": float64(0),
							}
							if spent < SESSION_PAUSE {
								time.Sleep(time.Duration(float64(SESSION_PAUSE)-spent) * time.Second)
							}
							for _, definition := range activeDefinitions {
								modes := 0
								log.Info("sending definition", definition.Name)

								if definition.TelemetryFlight.Acked {
									modes |= 1 << 2
								}
								if definition.TelemetryGcs.Acked {
									modes |= 1 << 3
								}

								meta["modes"] = float64(modes)

								setter := uavtalk.CreateObjectSetter(definition.Meta.Name, 0, meta)
								fcInChan <- *setter
								time.Sleep(10 * time.Millisecond)
							}

							for _, definition := range activeDefinitions {
								log.Info("sending definition", definition.Name)
								sendAsRotondeDefinitions(definition, client)
								sendAsRotondeDefinitions(definition.Meta, client)
							}
						}()

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
					start = time.Now()
					sessionManagingPacket := uavtalk.CreateSessionManagingPacket(0, 0)
					fcInChan <- sessionManagingPacket
					activeDefinitions = make([]*uavtalk.Definition, 0, 100)
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
		if event := toRotondePacket(p); event != nil {
			client.SendMessage(event)
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
	client.OnAction(func(i interface{}) bool {
		if p := toUAVTalkPacket(i); p != nil {
			fcInChan <- *p
		}
		return true
	})

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

func sendAsRotondeDefinitions(definition *uavtalk.Definition, client *client.Client) {
	name := strings.ToUpper(definition.Name)

	getter := rotonde.Definition{fmt.Sprintf("GET_%s", name), "action", false, []*rotonde.FieldDefinition{}}
	if definition.SingleInstance == false {
		getter.PushField("index", "number", "")
	}
	client.AddLocalDefinition(&getter)

	setter := rotonde.Definition{fmt.Sprintf("SET_%s", name), "action", false, []*rotonde.FieldDefinition{}}
	if definition.SingleInstance == false {
		setter.PushField("index", "number", "")
	}
	for _, field := range definition.Fields {
		setter.PushField(field.Name, field.Type, field.Units)
	}
	client.AddLocalDefinition(&setter)

	update := rotonde.Definition{name, "event", false, []*rotonde.FieldDefinition{}}
	if definition.SingleInstance == false {
		update.PushField("index", "number", "")
	}
	for _, field := range definition.Fields {
		update.PushField(field.Name, field.Type, field.Units)
	}
	client.AddLocalDefinition(&update)
}

func toRotondePacket(p uavtalk.Packet) interface{} {
	if p.Cmd != uavtalk.ObjectCmd && p.Cmd != uavtalk.ObjectCmdWithAck {
		return nil
	}
	name := strings.ToUpper(p.Definition.Name)
	event := rotonde.Event{name, p.Data}
	return event
}

func toUAVTalkPacket(i interface{}) *uavtalk.Packet {
	action, ok := i.(rotonde.Action)
	if ok == false {
		return nil
	}

	name := action.Identifier[4:]
	definition, err := uavtalk.AllDefinitions.GetDefinitionForName(name)
	if err != nil {
		log.Fatal(err)
	}

	var data map[string]interface{}
	var cmd uint8
	if strings.HasPrefix(action.Identifier, "GET_") {
		cmd = uavtalk.ObjectRequest
		data = map[string]interface{}{}
	} else if strings.HasPrefix(action.Identifier, "SET_") {
		if definition.Settings == true {
			cmd = uavtalk.ObjectCmdWithAck
		} else {
			cmd = uavtalk.ObjectCmd
		}
		data = action.Data
	}

	var instanceId uint16 = 0
	if definition.SingleInstance == false {
		index, ok := data["index"]
		if ok == false {
			instanceId = 0
		} else {
			delete(data, "index")
			value, ok := index.(float64)
			if ok == false {
				instanceId = 0
			} else {
				instanceId = uint16(value)
			}
		}
	}
	return uavtalk.NewPacket(definition, cmd, instanceId, data)
}
