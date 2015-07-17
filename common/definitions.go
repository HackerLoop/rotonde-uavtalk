package common

import (
	"errors"
	"fmt"
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

// GetDefinitionForName _
func (definitions Definitions) GetDefinitionForName(name string) (*Definition, error) {
	for _, definition := range definitions {
		if definition.Name == name {
			return definition, nil
		}
	}
	return nil, errors.New(fmt.Sprint(name, " Not found"))
}

// IsUniqueInstanceForObjectID an common is said unique when its number of instances is == 0 (which means, it is not an array)
func (definitions Definitions) IsUniqueInstanceForObjectID(objectID uint32) (bool, error) {
	definition, err := definitions.GetDefinitionForObjectID(objectID)
	if err != nil {
		return true, err
	}
	return definition.SingleInstance, nil
}

// FieldTypeInfo Taulabs defines its fields as type names, with a given size implicitely implied
type FieldTypeInfo struct {
	Index int
	Name  string
	Size  int
}

// TypeIndex holds a slice of *FieldTypeInfo, adds helper methods
type TypeIndex []*FieldTypeInfo

//
var TypeInfos = TypeIndex{
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
	for _, fieldTypeInfo := range TypeInfos {
		if fieldTypeInfo.Name == ts {
			return fieldTypeInfo, nil
		}
	}
	return nil, fmt.Errorf("Not found field type: %s", ts)
}

// FieldsSlice sortable slice of fields
type FieldsSlice []*FieldDefinition

// ByteLength returns the size in bytes of all the fields
func (fields FieldsSlice) ByteLength() int {
	length := 0
	for _, field := range fields {
		length += field.FieldTypeInfo.Size
	}
	return length
}

// FieldForName returns a fieldDefinition for a given name
func (fields FieldsSlice) FieldForName(name string) (*FieldDefinition, error) {
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
	Type  string `xml:"type,attr" json:"-"`

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
