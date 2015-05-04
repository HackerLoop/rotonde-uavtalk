package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s uavobject_directory", os.Args[0]))
	}

	loadUAVObjectDefinitions(os.Args[1])

	uavChan := make(chan *UAVTalkObject, 100)
	startUAVTalk(uavChan)
	startAsServer(uavChan)

	select {}

	return
}
