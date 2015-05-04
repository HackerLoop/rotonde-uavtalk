package main

import (
	"fmt"
	"log"
	"net/http"

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
		defer conn.Close()
		openUAVChan()

		go func() {
			for uavTalkObject := range uavChan {
				uavdef := getUAVObjectDefinitionForObjectID(uavTalkObject.objectId)

				if uavdef != nil {
					json, err := uavdef.uAVTalkToJSON(uavTalkObject.data)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Printf("%30s : %s\n", uavdef.Name, string(json))
					if err := conn.WriteMessage(websocket.TextMessage, json); err != nil {
						log.Fatal(err)
					}
				} else {
					//fmt.Printf("!!!!!!!!!!!! Not found : %d !!!!!!!!!!!!!!!!!\n", uavTalkObject.objectId)
				}
			}
		}()

		go func() {
			for {
				if _, _, err := conn.NextReader(); err != nil {
					log.Fatal(err)
					break
				}
			}
		}()

		log.Println("Treating messages")
		select {}
	})

	go http.ListenAndServe(":4242", nil)
	log.Println("Websocket server started")
}
