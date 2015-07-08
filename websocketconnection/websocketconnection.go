package websocketconnection

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/openflylab/bridge/dispatcher"
	"github.com/openflylab/bridge/uavobject"
)

// Packet is the JSON representation of an UAVTalk package, the Data field contains the UAVObject JSON representation.
// More infos at https://wiki.openpilot.org/display/WIKI/UAVTalk
// Warning: the link above might not be totally true in Taulabs, better read the code than the doc.
type Packet struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Start the websocket server, each peer connecting to this websocket will be added as connection to the dispatcher
func Start(d *dispatcher.Dispatcher, port int) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/uav", func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Connection received")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}

		defer conn.Close()

		startConnection(conn, d)
	})

	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	log.Println(fmt.Sprintf("Websocket server started on port %d", port))
}

func startConnection(conn *websocket.Conn, d *dispatcher.Dispatcher) {
	c := dispatcher.NewConnection()
	d.AddConnection(c)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		var jsonPacket []byte
		var err error
		var packet Packet

		for dispatcherPacket := range c.InChan {
			switch data := dispatcherPacket.(type) {
			case dispatcher.Update:
				packet = Packet{Type: "update", Data: data}
			case dispatcher.Request:
				packet = Packet{Type: "req", Data: data}
			case uavobject.Definition:
				packet = Packet{Type: "def", Data: data}
			}

			jsonPacket, err = json.Marshal(packet)
			if err != nil {
				log.Warning(err)
			}

			if err := conn.WriteMessage(websocket.TextMessage, jsonPacket); err != nil {
				log.Warning(err)
				return
			}

			// TODO log
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		var dispatcherPacket interface{}

		for {
			messageType, reader, err := conn.NextReader()
			if err != nil {
				log.Println(err)
				return
			}
			if messageType == websocket.TextMessage {
				packet := Packet{}
				decoder := json.NewDecoder(reader)
				if err := decoder.Decode(&packet); err != nil {
					log.Warning(err)
					continue
				}

				switch packet.Type {
				case "update":
					update := dispatcher.Update{}
					mapstructure.Decode(packet.Data, &update)
					dispatcherPacket = update
				case "req":
					request := dispatcher.Request{}
					mapstructure.Decode(packet.Data, &request)
					dispatcherPacket = request
				case "cmd":
					// TODO available websocket command: filter
				case "def":
					definition := uavobject.Definition{}
					mapstructure.Decode(packet.Data, &definition)
					dispatcherPacket = definition
				}
				// TODO log

				c.OutChan <- dispatcherPacket
			}
		}
	}()

	log.Println("Treating messages")
	wg.Wait()
}
