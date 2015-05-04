package main

import (
	//"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
)

/**
 * uavObjectDefinitions storage
 */
var uavObjectDefinitions []*UAVObjectDefinition

/**
 * Utils
 */
type FieldTypeInfo struct {
	index int
	name  string
	size  int
}

type TypeIndex []*FieldTypeInfo

var typeInfos = TypeIndex{
	&FieldTypeInfo{0, "int8", 1},
	&FieldTypeInfo{1, "int16", 2},
	&FieldTypeInfo{2, "int32", 4},
	&FieldTypeInfo{3, "uint8", 1},
	&FieldTypeInfo{4, "uint16", 2},
	&FieldTypeInfo{5, "uint32", 4},
	&FieldTypeInfo{6, "float", 4},
	&FieldTypeInfo{7, "enum", 1},
}

func (t TypeIndex) fieldTypeForString(ts string) (*FieldTypeInfo, error) {
	for _, fieldTypeInfo := range typeInfos {
		if fieldTypeInfo.name == ts {
			return fieldTypeInfo, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("Not found field type: %s", ts))
}

// sorted slice of fields
type FieldSlice []*UAVObjectFieldDefinition

func (fields FieldSlice) fieldForName(name string) (*UAVObjectFieldDefinition, error) {
	for _, field := range fields {
		if field.Name == name {
			return field, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("Not found field name: %s", name))
}

func (fields FieldSlice) Len() int {
	return len(fields)
}

func (fields FieldSlice) Less(i, j int) bool {
	return fields[i].fieldTypeInfo.size > fields[j].fieldTypeInfo.size
}

func (fields FieldSlice) Swap(i, j int) {
	fields[i], fields[j] = fields[j], fields[i]
}

// uavObjectDefinitions models
type UAVObjectFieldDefinition struct {
	Name  string `xml:"name,attr"`
	Units string `xml:"units,attr"`
	Type  string `xml:"type,attr"`

	fieldTypeInfo *FieldTypeInfo

	Elements         int      `xml:"elements,attr"`
	ElementNamesAttr string   `xml:"elementnames,attr" json:"-"`
	ElementNames     []string `xml:"elementnames>elementname"`
	OptionsAttr      string   `xml:"options,attr" json:"-"`
	Options          []string `xml:"options>option"`
	DefaultValue     string   `xml:"defaultvalue,attr"`

	CloneOf string `xml:"cloneof,attr"`
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

	Fields FieldSlice `xml:"field"`
}

func newUAVObjectDefinition(filePath string) (*UAVObjectDefinition, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(file)

	var content = &struct {
		UAVObject *UAVObjectDefinition `xml:"object"`
	}{}
	decoder.Decode(content)

	uavObject := content.UAVObject

	// fields post process
	for _, field := range uavObject.Fields {
		if len(field.CloneOf) != 0 {
			continue
		}
		field.Elements = 1
		if len(field.ElementNamesAttr) > 0 {
			field.ElementNames = strings.Split(field.ElementNamesAttr, ",")
			field.Elements = len(field.ElementNames)
		} else if len(field.ElementNames) > 0 {
			field.Elements = len(field.ElementNames)
		}

		if len(field.OptionsAttr) > 0 {
			field.Options = strings.Split(field.OptionsAttr, ",")
		}

		field.fieldTypeInfo, err = typeInfos.fieldTypeForString(field.Type)
		if err != nil {
			return nil, err
		}
	}

	// create clones
	for _, field := range uavObject.Fields {
		if len(field.CloneOf) != 0 {
			clonedField, err := uavObject.Fields.fieldForName(field.CloneOf)
			if err != nil {
				return nil, err
			}
			name, cloneOf := field.Name, field.CloneOf
			*field = *clonedField
			field.Name, field.CloneOf = name, cloneOf
		}
	}

	sort.Sort(uavObject.Fields)

	uavObject.calculateId()

	return uavObject, nil
}

// exported functions
func getUAVObjectDefinitionForObjectID(objectID uint32) (*UAVObjectDefinition, error) {
	for _, uavdef := range uavObjectDefinitions {
		if uavdef.ObjectID == objectID {
			return uavdef, nil
		}
	}
	return nil, errors.New(fmt.Sprint(objectID, " Not found"))
}

func loadUAVObjectDefinitions(dir string) error {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, fileInfo := range fileInfos {
		filePath := fmt.Sprintf("%s%s", dir, fileInfo.Name())
		uavdef, err := newUAVObjectDefinition(filePath)
		if err != nil {
			log.Fatal(err)
		}
		uavObjectDefinitions = append(uavObjectDefinitions, uavdef)
	}
	return nil
}
