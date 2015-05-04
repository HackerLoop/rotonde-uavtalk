package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
)

func (field *UAVObjectFieldDefinition) readFromUAVTalk(reader *bytes.Reader) (interface{}, error) {
	typeInfo := field.fieldTypeInfo
	switch typeInfo.name {
	case "int8":
		var result int8
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "int16":
		var result int16
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "int32":
		var result int32
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "uint8":
		var result uint8
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "uint16":
		var result uint16
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "uint32":
		var result uint32
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "float":
		var result float32
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "enum":
		var result int8
		if err := binary.Read(reader, binary.LittleEndian, &result); err != nil {
			return nil, err
		}
		return field.Options[result], nil
	}
	return nil, errors.New("Could not read from typeInfo.")
}

func (field *UAVObjectFieldDefinition) uAVTalkToInterface(reader *bytes.Reader) (interface{}, error) {
	var result interface{}
	if field.Elements > 1 {
		resultArray := make([]interface{}, field.Elements)
		for i := 0; i < field.Elements; i++ {
			value, err := field.readFromUAVTalk(reader)
			if err != nil {
				return nil, err
			}
			resultArray[i] = value
		}
		result = resultArray
	} else {
		value, err := field.readFromUAVTalk(reader)
		if err != nil {
			return nil, err
		}
		result = value
	}
	return result, nil
}

func (uavdef *UAVObjectDefinition) uAVTalkToJSON(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	result := make(map[string]interface{})
	for _, field := range uavdef.Fields {
		value, err := field.uAVTalkToInterface(reader)
		if err != nil {
			return "", err
		}
		result[field.Name] = value
	}

	val, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(val), err
}

func (*UAVObjectDefinition) jSONtoUAVTalk(json string) []byte {
	return nil
}
