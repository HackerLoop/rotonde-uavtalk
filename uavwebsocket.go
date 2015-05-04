package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

func startAsServer(uavChan chan *UAVTalkObject) {

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/uav", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Connection received")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer closeUAVChan()
		defer conn.Close()
		openUAVChan()

		var wg sync.WaitGroup
		outCloseChan := make(chan bool)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for uavTalkObject := range uavChan {
				select {
				case <-outCloseChan:
					return
				default:
				}

				uavdef := getUAVObjectDefinitionForObjectID(uavTalkObject.objectId)

				if uavdef != nil {
					json, err := uavdef.uAVTalkToJSON(uavTalkObject.data)
					if err != nil {
						log.Fatal(err)
					}
					//log.Printf("%30s : %s\n", uavdef.Name, string(json))
					if err := conn.WriteMessage(websocket.TextMessage, json); err != nil {
						log.Println(err)
						return
					}
				} else {
					//log.Printf("!!!!!!!!!!!! Not found : %d !!!!!!!!!!!!!!!!!\n", uavTalkObject.objectId)
				}
			}
		}()

		inCloseChan := make(chan bool)
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-inCloseChan:
					return
				default:
				}
				messageType, reader, err := conn.NextReader()
				if err != nil {
					log.Println(err)
					return
				}
				if messageType == websocket.TextMessage {
					content := make(map[string]interface{})
					decoder := json.NewDecoder(reader)
					if err := decoder.Decode(&content); err != nil {
						log.Println(err)
					} else {
						log.Println(content)
					}
				}
			}
		}()

		log.Println("Treating messages")
		wg.Wait()
	})

	go http.ListenAndServe(":4242", nil)
	log.Println("Websocket server started")
}
