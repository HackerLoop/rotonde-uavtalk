package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s uavobject_directory/", os.Args[0]))
	}

	port := flag.Int("port", 4224, "port the websocket will listen on")
	flag.Parse()

	loadUAVObjectDefinitions(flag.Args()[0])

	uavChan := make(chan *UAVTalkObject, 100)
	jsonChan := make(chan *UAVTalkObject, 100)
	startUAVTalk(uavChan, jsonChan)
	startAsServer(uavChan, jsonChan, *port)

	select {}
}
