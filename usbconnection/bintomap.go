package usbconnection

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/openflylab/bridge/uavobject"
	//"log"
)

func readFromUAVTalk(field *uavobject.FieldDefinition, reader *bytes.Reader) (interface{}, error) {
	typeInfo := field.FieldTypeInfo
	var result interface{}
	switch typeInfo.Name {
	case "int8":
		result = new(uint8)
	case "int16":
		result = new(uint16)
	case "int32":
		result = new(int32)
	case "uint8":
		result = new(uint8)
	case "uint16":
		result = new(uint16)
	case "uint32":
		result = new(uint32)
	case "float":
		result = new(float32)
	case "enum":
		result = new(uint8)
	default:
		return nil, errors.New("Could not read from typeInfo.")
	}

	if err := binary.Read(reader, binary.LittleEndian, result); err != nil {
		return nil, err
	}

	if typeInfo.Name == "enum" {
		result = field.Options[uint8(*(result.(*uint8)))] // haha
	}

	/*switch typeInfo.name {
	case "int8":
		log.Println(field.Name, *result.(*int8))
	case "int16":
		log.Println(field.Name, *result.(*int16))
	case "int32":
		log.Println(field.Name, *result.(*int32))
	case "uint8":
		log.Println(field.Name, *result.(*uint8))
	case "uint16":
		log.Println(field.Name, *result.(*uint16))
	case "uint32":
		log.Println(field.Name, *result.(*uint32))
	case "float":
		log.Println(field.Name, *result.(*float32))
	case "enum":
		log.Println(field.Name, result.(string))
	}*/
	return result, nil
}

func uAVTalkToInterface(field *uavobject.FieldDefinition, reader *bytes.Reader) (interface{}, error) {
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

func uAVTalkToMap(uavdef *uavobject.Definition, data []byte) (map[string]interface{}, error) {
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
