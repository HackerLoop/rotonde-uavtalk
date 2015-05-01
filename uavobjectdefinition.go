package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
)

var definitions []*UAVObjectDefinition

type UAVObjectFieldDefinition struct {
	Name         string `xml:"name,attr"`
	Units        string `xml:"units,attr"`
	Type         string `xml:"type,attr"`
	Elements     int    `xml:"elements,attr"`
	ElementNames string `xml:"elementnames,attr"`
	DefaultValue string `xml:"defaultvalue,attr"`
}

type UAVObjectDefinition struct {
	Name           string `xml:"name,attr"`
	Description    string `xml:"description"`
	SingleInstance bool   `xml:"singleinstance,attr"`
	Settings       bool   `xml:"settings,attr"`
	Category       string `xml:"category,attr"`

	ObjectID uint32

	Access struct {
		Gcs    string `xml:"gcs,attr"`
		Flight string `xml:"flight,attr"`
	} `xml:"access"`

	TelemetryGcs struct {
		Acked      string `xml:"acked,attr"`
		UpdateMode string `xml:"updatemode,attr"`
		Period     string `xml:"period,attr"`
	} `xml:"telemetrygcs"`

	TelemetryFlight struct {
		Acked      string `xml:"acked,attr"`
		UpdateMode string `xml:"updatemode,attr"`
		Period     string `xml:"period,attr"`
	} `xml:"telemetryflight"`

	Logging struct {
		UpdateMode string `xml:"updatemode,attr"`
		Period     string `xml:"period,attr"`
	} `xml:"logging"`

	Fields []UAVObjectFieldDefinition `xml:"field"`
}

func (*UAVObjectDefinition) jsonCreateObject() string {
	return ""
}

func (*UAVObjectDefinition) uAVTalkToJSON([]byte) string {
	return ""
}

func (*UAVObjectDefinition) jSONtoUAVTalk(string) []byte {
	return nil
}

func loadUAVObjectDefinitions(dir string) error {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, fileInfo := range fileInfos {
		filePath := fmt.Sprintf("%s%s", dir, fileInfo.Name())
		parseUAVObjectDefinition(filePath)
	}
	return nil
}

func parseUAVObjectDefinition(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	decoder := xml.NewDecoder(file)

	var content = &struct {
		UAVObject *UAVObjectDefinition `xml:"object"`
	}{}
	decoder.Decode(content)
	definitions = append(definitions, content.UAVObject)

	//fmt.Println(content.UAVObject)
	return nil
}
