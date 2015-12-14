package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/openskybot/rotonde-uavtalk/uavtalk"
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

func main() {
	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s common_directory/", os.Args[0]))
	}

	fcInChan := make(chan uavtalk.Packet, 100)
	fcOutChan := make(chan uavtalk.Packet, 100)

	go uavtalk.Start(fcInChan, fcOutChan, os.Args[1])

	root := handlers.NewHandlerManager(chanCast(fcOutChan), handlers.PassAll, handlers.Noop, handlers.Noop)

	auth := handlers.NewHandlerManager(root.NewOutChan(10), func(i interface{}) (interface{}, bool) {
		p := i.(uavtalk.Packet)

		return i, authPackets.contains(p.Definition.Name)
	}, handlers.Noop, handlers.Noop)
	auth.Attach(func(i interface{}) bool {
		p := i.(uavtalk.Packet)
		log.Info("AUTH", p)
		return true
	})

	stream := handlers.NewHandlerManager(root.NewOutChan(10), func(i interface{}) (interface{}, bool) {
		p := i.(uavtalk.Packet)

		return i, authPackets.contains(p.Definition.Name) == false
	}, handlers.Noop, handlers.Noop)
	stream.Attach(func(i interface{}) bool {
		//p := i.(uavtalk.Packet)
		return true
	})

	select {}
}
