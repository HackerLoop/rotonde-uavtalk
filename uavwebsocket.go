package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type JSONPackage struct {
	Name       string
	Cmd        uint8
	ObjectId   uint32
	InstanceId uint16
	Data       map[string]interface{}
}

func startAsServer(uavChan chan *UAVTalkObject, jsonChan chan *UAVTalkObject) {

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

		websocket.WriteJSON(conn, uavObjectDefinitions)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for uavTalkObject := range uavChan {
				uavdef, err := getUAVObjectDefinitionForObjectID(uavTalkObject.objectId)
				if err != nil {
					//log.Println(err)
					continue
				}

				data, err := uavdef.uAVTalkToMap(uavTalkObject.data)
				if err != nil {
					log.Fatal(err)
				}

				jsonObject := JSONPackage{uavdef.Name, uavTalkObject.cmd, uavdef.ObjectID, uavTalkObject.instanceId, data}

				json, err := json.Marshal(&jsonObject)
				if err != nil {
					log.Fatal(err)
				}
				//log.Printf("%30s : %s\n", uavdef.Name, string(json))
				if err := conn.WriteMessage(websocket.TextMessage, json); err != nil {
					log.Println(err)
					return
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				messageType, reader, err := conn.NextReader()
				if err != nil {
					log.Println(err)
					return
				}
				if messageType == websocket.TextMessage {
					content := JSONPackage{}
					decoder := json.NewDecoder(reader)
					if err := decoder.Decode(&content); err != nil {
						log.Println(err)
						continue
					}

					uavdef, err := getUAVObjectDefinitionForObjectID(content.ObjectId)
					if err != nil {
						log.Println(err)
						continue
					}

					data, err := uavdef.mapToUAVTalk(content.Data)
					if err != nil {
						log.Println(data)
						continue
					}

					uavTalkObject, err := newUAVTalkObject(content.Cmd, content.ObjectId, content.InstanceId, data)
					if err != nil {
						log.Println(err)
						continue
					}
					jsonChan <- uavTalkObject
				}
			}
		}()

		log.Println("Treating messages")
		wg.Wait()
	})

	go http.ListenAndServe(":4242", nil)
	log.Println("Websocket server started")
}
