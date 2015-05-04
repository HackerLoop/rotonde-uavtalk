package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal(fmt.Sprintf("Usage: %s uavobject_directory [uavobjectname]", os.Args[0]))
	}

	uavObjectName := "all"
	if len(os.Args) > 2 {
		uavObjectName = os.Args[2]
	}

	loadUAVObjectDefinitions(os.Args[1])

	c := startUAVTalk()

	for uavTalkObject := range c {
		uavdef := getUAVObjectDefinitionForObjectID(uavTalkObject.objectId)

		if uavdef != nil {
			if uavObjectName != "all" && uavdef.Name != uavObjectName {
				continue
			}
			//fmt.Println(uavTalkObject)
			//fmt.Println(uavdef)

			json, err := uavdef.uAVTalkToJSON(uavTalkObject.data)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%30s : %s\n", uavdef.Name, json)
		} else {
			//fmt.Printf("!!!!!!!!!!!! Not found : %d !!!!!!!!!!!!!!!!!\n", uavTalkObject.objectId)
		}
		//fmt.Println("")
	}

	return
}
