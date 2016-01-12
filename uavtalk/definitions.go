package uavtalk

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

/**
 * this is currently shared between the uavtalk connection for parsing Taulabs's XML definitions,
 * and the dispatcher as definition packets used to expose available definitions.
 * This is going to move to the dispatcher connection,
 * and a Definition struct will be defined in the dispatcher.
 */

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
		if strings.ToLower(definition.Name) == strings.ToLower(name) {
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
		length += field.FieldTypeInfo.Size * field.Elements
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
	Type  string `xml:"type,attr" json:"type"`
	Units string `xml:"units,attr" json:"units"`

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

	ObjectID uint32 `json:"id" mapstructure:"id"`

	MetaFor *Definition `xml:"-" json:"-"`
	Meta    *Definition `xml:"-" json:"-"`

	Access struct {
		Gcs    string `xml:"gcs,attr" json:"-"`
		Flight string `xml:"flight,attr" json:"-"`
	} `xml:"access" json:"-"`

	TelemetryGcs struct {
		Acked      bool   `xml:"acked,attr" json:"-"`
		UpdateMode string `xml:"updatemode,attr" json:"-"`
		Period     string `xml:"period,attr" json:"-"`
	} `xml:"telemetrygcs" json:"-"`

	TelemetryFlight struct {
		Acked      bool   `xml:"acked,attr" json:"-"`
		UpdateMode string `xml:"updatemode,attr" json:"-"`
		Period     string `xml:"period,attr" json:"-"`
	} `xml:"telemetryflight" json:"-"`

	Logging struct {
		UpdateMode string `xml:"updatemode,attr" json:"-"`
		Period     string `xml:"period,attr" json:"-"`
	} `xml:"logging" json:"-"`

	Fields FieldsSlice `xml:"field" json:"fields"`
}

// FinishSetup has to be called on a definition once the fields are setup
func (definition *Definition) FinishSetup() error {
	var err error
	// fields post process
	for _, field := range definition.Fields {
		if len(field.CloneOf) != 0 {
			continue
		}

		if field.Elements == 0 {
			field.Elements = 1
		}

		if len(field.ElementNamesAttr) > 0 {
			field.ElementNames = strings.Split(sanitizeListString(field.ElementNamesAttr), ",")
			field.Elements = len(field.ElementNames)
		} else if len(field.ElementNames) > 0 {
			field.Elements = len(field.ElementNames)
		}

		if len(field.OptionsAttr) > 0 {
			field.Options = strings.Split(sanitizeListString(field.OptionsAttr), ",")
		}

		field.FieldTypeInfo, err = TypeInfos.FieldTypeForString(field.Type)
		if err != nil {
			return err
		}
	}

	// create clones
	for _, field := range definition.Fields {
		if len(field.CloneOf) != 0 {
			clonedField, err := definition.Fields.FieldForName(field.CloneOf)
			if err != nil {
				return err
			}
			name, cloneOf := field.Name, field.CloneOf
			*field = *clonedField
			field.Name, field.CloneOf = name, cloneOf
		}
	}

	sort.Stable(definition.Fields)
	return nil
}

func sanitizeListString(s string) string {
	s = strings.Replace(s, ", ", ",", -1)
	s = strings.Replace(s, "\n", "", -1)
	s = strings.Replace(s, "\t", "", -1)
	return s
}

func NewMetaDefinition(parent *Definition) (*Definition, error) {
	if parent.MetaFor != nil {
		return nil, fmt.Errorf("Meta definition cannot be created for meta definitions")
	}

	meta := &Definition{}
	meta.Name = fmt.Sprintf("%s%s", parent.Name, "Meta")
	meta.Description = fmt.Sprintf("Meta for: \n%s", parent.Description)
	meta.SingleInstance = true
	meta.Settings = false

	meta.ObjectID = parent.ObjectID + 1

	meta.MetaFor = parent
	parent.Meta = meta

	meta.Fields = append(meta.Fields, &FieldDefinition{Name: "modes", Units: "boolean", Type: "uint8"})
	meta.Fields = append(meta.Fields, &FieldDefinition{Name: "periodFlight", Units: "ms", Type: "uint16"})
	meta.Fields = append(meta.Fields, &FieldDefinition{Name: "periodGCS", Units: "ms", Type: "uint16"})
	meta.Fields = append(meta.Fields, &FieldDefinition{Name: "periodLog", Units: "ms", Type: "uint16"})

	if err := meta.FinishSetup(); err != nil {
		return nil, err
	}

	return meta, nil
}
