package uavtalkconnection

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/openflylab/bridge/common"
)

func readFromUAVTalk(field *common.FieldDefinition, reader *bytes.Reader) (interface{}, error) {
	typeInfo := field.FieldTypeInfo
	var result interface{}
	switch typeInfo.Name {
	case "int8":
		tmp := uint8(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "int16":
		tmp := uint16(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "int32":
		tmp := int32(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "uint8":
		tmp := uint8(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "uint16":
		tmp := uint16(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "uint32":
		tmp := uint32(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "float":
		tmp := float32(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	case "enum":
		tmp := uint8(0)
		if err := binary.Read(reader, binary.LittleEndian, &tmp); err != nil {
			return nil, err
		}
		result = tmp
	default:
		return nil, errors.New("Could not read from typeInfo.")
	}

	if typeInfo.Name == "enum" {
		result = field.Options[result.(uint8)] // haha
	}
	return result, nil
}

func uAVTalkToInterface(field *common.FieldDefinition, reader *bytes.Reader) (interface{}, error) {
	var result interface{}
	if field.Elements > 1 && len(field.ElementNames) == 0 {
		resultArray := make([]interface{}, field.Elements)
		for i := 0; i < field.Elements; i++ {
			value, err := readFromUAVTalk(field, reader)
			if err != nil {
				return nil, err
			}
			resultArray[i] = value
		}
		result = resultArray
	} else if field.Elements > 1 && len(field.ElementNames) > 0 {
		resultMap := make(map[string]interface{}, field.Elements)
		for i := 0; i < field.Elements; i++ {
			value, err := readFromUAVTalk(field, reader)
			if err != nil {
				return nil, err
			}
			resultMap[field.ElementNames[i]] = value
		}
		result = resultMap
	} else {
		value, err := readFromUAVTalk(field, reader)
		if err != nil {
			return nil, err
		}
		result = value
	}
	return result, nil
}

func uAVTalkToMap(uavdef *common.Definition, data []byte) (map[string]interface{}, error) {
	reader := bytes.NewReader(data)
	result := make(map[string]interface{})

	for _, field := range uavdef.Fields {
		value, err := uAVTalkToInterface(field, reader)
		if err != nil {
			return nil, err
		}
		result[field.Name] = value
	}

	return result, nil
}
