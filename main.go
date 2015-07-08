package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/dispatcher"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s uavobject_directory/", os.Args[0]))
	}

	port := flag.Int("port", 4224, "port the websocket will listen on")
	flag.Parse()

	d := dispatcher.NewDispatcher()

	select {}
}
