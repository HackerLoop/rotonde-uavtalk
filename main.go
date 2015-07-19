package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/openskybot/skybot-router/dispatcher"
	"github.com/openskybot/skybot-router/uavtalkconnection"
	"github.com/openskybot/skybot-router/websocketconnection"
)

func main() {
	runtime.GOMAXPROCS(1) //runtime.NumCPU())

	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s common_directory/", os.Args[0]))
	}

	port := flag.Int("port", 4224, "port the websocket will listen on")
	flag.Parse()

	d := dispatcher.NewDispatcher()

	go uavtalkconnection.Start(d, os.Args[1])

	go websocketconnection.Start(d, *port)

	go d.Start()

	select {}
}
