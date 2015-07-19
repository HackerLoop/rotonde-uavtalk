package websocketconnection

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/openskybot/skybot-router/common"
	"github.com/openskybot/skybot-router/dispatcher"
)

// Packet is the JSON representation of an UAVTalk package, the Data field contains the common JSON representation.
// More infos at https://wiki.openpilot.org/display/WIKI/UAVTalk
// Warning: the link above might not be totally true in Taulabs, better read the code than the doc.
type Packet struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Start the websocket server, each peer connecting to this websocket will be added as a connection to the dispatcher
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
	select {}
}

func startConnection(conn *websocket.Conn, d *dispatcher.Dispatcher) {
	c := dispatcher.NewConnection()
	d.AddConnection(c)
	defer c.Close()

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
				packet = Packet{Type: "update", Payload: data}
			case dispatcher.Request:
				packet = Packet{Type: "req", Payload: data}
			case common.Definition:
				packet = Packet{Type: "def", Payload: data}
			default:
				log.Info("Oops unknown packet: ", packet)
			}

			jsonPacket, err = json.Marshal(packet)
			if err != nil {
				log.Warning(err)
			}

			if err := conn.WriteMessage(websocket.TextMessage, jsonPacket); err != nil {
				log.Warning(err)
				return
			}
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
					mapstructure.Decode(packet.Payload, &update)
					dispatcherPacket = update
				case "req":
					request := dispatcher.Request{}
					mapstructure.Decode(packet.Payload, &request)
					dispatcherPacket = request
				case "sub":
					subscription := dispatcher.Subscription{}
					mapstructure.Decode(packet.Payload, &subscription)
					dispatcherPacket = subscription
				case "unsub":
					unsubscription := dispatcher.Unsubscription{}
					mapstructure.Decode(packet.Payload, &unsubscription)
					dispatcherPacket = unsubscription
				case "def":
					definition := common.Definition{}
					mapstructure.Decode(packet.Payload, &definition)
					dispatcherPacket = definition
				}

				c.OutChan <- dispatcherPacket
			}
		}
	}()

	log.Println("Treating messages")
	wg.Wait()
}
