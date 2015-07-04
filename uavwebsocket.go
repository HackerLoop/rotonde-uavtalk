package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

// JSONPackage is the JSON representation of an UAVTalk package, the Data field contains the UAVObject JSON representation.
// More infos at https://wiki.openpilot.org/display/WIKI/UAVTalk
// Warning: the link above might not be totally true in Taulabs, better read the code than the doc.
type JSONPackage struct {
	Name       string                 `json:"name"`
	Cmd        uint8                  `json:"cmd"`
	ObjectId   uint32                 `json:"objectId"`
	InstanceId uint16                 `json:"instanceId"`
	Data       map[string]interface{} `json:"data"`
}

func startAsServer(uavChan chan *UAVTalkObject, jsonChan chan *UAVTalkObject, port int) {

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
					log.Warning(err)
					continue
				}

				var data map[string]interface{}
				if uavTalkObject.cmd == 0 || uavTalkObject.cmd == 2 {
					data, err = uavdef.uAVTalkToMap(uavTalkObject.data)
					if err != nil {
						log.Warning(err)
						return
					}
				}

				jsonObject := JSONPackage{uavdef.Name, uavTalkObject.cmd, uavdef.ObjectID, uavTalkObject.instanceId, data}
				json, err := json.Marshal(&jsonObject)
				if err != nil {
					log.Warning(err)
					continue
				}

				if err := conn.WriteMessage(websocket.TextMessage, json); err != nil {
					log.Warning(err)
					return
				}

				log.WithFields(log.Fields{
					"ObjectID": uavdef.ObjectID,
					"Name":     uavdef.Name,
				}).Debug("UAVObject to websocket client")
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
						log.Warning(err)
						continue
					}

					uavdef, err := getUAVObjectDefinitionForObjectID(content.ObjectId)
					if err != nil {
						log.Warning(err)
						continue
					}

					var data []byte
					if content.Cmd == 0 || content.Cmd == 2 {
						data, err = uavdef.mapToUAVTalk(content.Data)
						if err != nil {
							log.Warning(data)
							continue
						}
					}

					uavTalkObject, err := newUAVTalkObject(content.Cmd, content.ObjectId, content.InstanceId, data)
					if err != nil {
						log.Warning(err)
						continue
					}

					log.WithFields(log.Fields{
						"ObjectID": uavdef.ObjectID,
						"Name":     uavdef.Name,
					}).Debug("UAVObject from websocket client")

					jsonChan <- uavTalkObject
				}
			}
		}()

		log.Println("Treating messages")
		wg.Wait()
	})

	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	log.Println(fmt.Sprintf("Websocket server started on port %d", port))
}
