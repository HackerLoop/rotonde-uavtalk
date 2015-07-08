package uavobject

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

// Definitions is a slice of Definition, adds findBy
type Definitions []*Definition

// GetDefinitionForObjectID _
func (definitions Definitions) GetDefinitionForObjectID(objectID uint32) (*Definition, error) {
	for _, definition := range definitions {
		if definition.ObjectID == objectID {
			return definition, nil
		}
	}
	return nil, errors.New(fmt.Sprint(objectID, " Not found"))
}

// IsUniqueInstanceForObjectID an UAVObject is said unique when its number of instances is == 0 (which means, it is not an array)
func (definitions Definitions) IsUniqueInstanceForObjectID(objectID uint32) (bool, error) {
	definition, err := definitions.GetDefinitionForObjectID(objectID)
	if err != nil {
		return true, err
	}
	return definition.SingleInstance, nil
}

// NewDefinitions loads all xml files from a directory
func NewDefinitions(dir string) (Definitions, error) {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	definitions := make([]*Definition, 10)
	for _, fileInfo := range fileInfos {
		filePath := fmt.Sprintf("%s%s", dir, fileInfo.Name())
		definition, err := NewDefinition(filePath)
		if err != nil {
			log.Fatal(err)
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

// FieldTypeInfo Taulabs defines its fields as type names, with a given size implicitely implied
type FieldTypeInfo struct {
	Index int
	Name  string
	Size  int
}

// TypeIndex holds a slice of *FieldTypeInfo, adds helper methods
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

// FieldTypeForString _
func (t TypeIndex) FieldTypeForString(ts string) (*FieldTypeInfo, error) {
	for _, fieldTypeInfo := range typeInfos {
		if fieldTypeInfo.Name == ts {
			return fieldTypeInfo, nil
		}
	}
	return nil, fmt.Errorf("Not found field type: %s", ts)
}

// FieldsSlice sortable slice of fields
type FieldsSlice []*FieldDefinition

func (fields FieldsSlice) fieldForName(name string) (*FieldDefinition, error) {
	for _, field := range fields {
		if field.Name == name {
			return field, nil
		}
	}
	return nil, fmt.Errorf("Not found field name: %s", name)
}

func (fields FieldsSlice) Len() int {
	return len(fields)
}

func (fields FieldsSlice) Less(i, j int) bool {
	return fields[i].FieldTypeInfo.Size > fields[j].FieldTypeInfo.Size
}

func (fields FieldsSlice) Swap(i, j int) {
	fields[i], fields[j] = fields[j], fields[i]
}

// FieldDefinition _
type FieldDefinition struct {
	Name  string `xml:"name,attr" json:"name"`
	Units string `xml:"units,attr" json:"units"`
	Type  string `xml:"type,attr" json:"type"`

	FieldTypeInfo *FieldTypeInfo

	Elements         int      `xml:"elements,attr" json:"elements"`
	ElementNamesAttr string   `xml:"elementnames,attr" json:"-"`
	ElementNames     []string `xml:"elementnames>elementname" json:"elementsName"`
	OptionsAttr      string   `xml:"options,attr" json:"-"`
	Options          []string `xml:"options>option" json:"options"`
	DefaultValue     string   `xml:"defaultvalue,attr" json:"defaultValue"`

	CloneOf string `xml:"cloneof,attr" json:"cloneOf"`
}

// Definition _
type Definition struct {
	Name           string `xml:"name,attr" json:"name"`
	Description    string `xml:"description" json:"description"`
	SingleInstance bool   `xml:"singleinstance,attr" json:"singleInstance"`
	Settings       bool   `xml:"settings,attr" json:"settings"`
	Category       string `xml:"category,attr" json:"category"`

	ObjectID uint32 `json:"id"`

	Access struct {
		Gcs    string `xml:"gcs,attr" json:"gcs"`
		Flight string `xml:"flight,attr" json:"flight"`
	} `xml:"access" json:"access"`

	TelemetryGcs struct {
		Acked      bool   `xml:"acked,attr" json:"acked"`
		UpdateMode string `xml:"updatemode,attr" json:"updateMode"`
		Period     string `xml:"period,attr" json:"period"`
	} `xml:"telemetrygcs" json:"telemetryGcs"`

	TelemetryFlight struct {
		Acked      bool   `xml:"acked,attr" json:"acked"`
		UpdateMode string `xml:"updatemode,attr" json:"updateMode"`
		Period     string `xml:"period,attr" json:"period"`
	} `xml:"telemetryflight" json:"telemetryFlight"`

	Logging struct {
		UpdateMode string `xml:"updatemode,attr" json:"updateMode"`
		Period     string `xml:"period,attr" json:"period"`
	} `xml:"logging" json:"logging"`

	Fields FieldsSlice `xml:"field" json:"fields"`
}

// NewDefinition create an Definition from an xml file.
func NewDefinition(filePath string) (*Definition, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(file)

	var content = &struct {
		UAVObject *Definition `xml:"object"`
	}{}
	decoder.Decode(content)

	uavObject := content.UAVObject

	// fields post process
	for _, field := range uavObject.Fields {
		if len(field.CloneOf) != 0 {
			continue
		}

		if field.Elements == 0 {
			field.Elements = 1
		}

		if len(field.ElementNamesAttr) > 0 {
			field.ElementNames = strings.Split(field.ElementNamesAttr, ",")
			field.Elements = len(field.ElementNames)
		} else if len(field.ElementNames) > 0 {
			field.Elements = len(field.ElementNames)
		}

		if len(field.OptionsAttr) > 0 {
			field.Options = strings.Split(field.OptionsAttr, ",")
		}

		field.FieldTypeInfo, err = typeInfos.FieldTypeForString(field.Type)
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

	sort.Stable(uavObject.Fields)

	uavObject.calculateID()

	return uavObject, nil
}
