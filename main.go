package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/dispatcher"
	"github.com/openflylab/bridge/usbconnection"
	"github.com/openflylab/bridge/websocketconnection"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s uavobject_directory/", os.Args[0]))
	}

	port := flag.Int("port", 4224, "port the websocket will listen on")
	flag.Parse()

	d := dispatcher.NewDispatcher()

	go usbconnection.Start(d, os.Args[1])

	go websocketconnection.Start(d, *port)

	go d.Start()

	select {}
}
