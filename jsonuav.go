package main

import (
	"bytes"
	"encoding/binary"
	"errors"
)

/**
 * UAVObjectFieldDefinition
 */

func (field *UAVObjectFieldDefinition) writeToUAVTalk(writer *bytes.Buffer, value interface{}) error {
	typeInfo := field.fieldTypeInfo
	var result interface{}
	switch typeInfo.name {
	case "int8":
		result = value.(uint8)
	case "int16":
		result = value.(uint16)
	case "int32":
		result = value.(int32)
	case "uint8":
		result = value.(uint8)
	case "uint16":
		result = value.(uint16)
	case "uint32":
		result = value.(uint32)
	case "float":
		result = float32(value.(float64))
	case "enum":
		result = value.(uint8)
	}
	if result == nil {
		return errors.New("Could not read from typeInfo.")
	}
	if err := binary.Write(writer, binary.LittleEndian, result); err != nil {
		return err
	}

	if typeInfo.name == "enum" {
		result = field.Options[*(result.(*uint8))] // haha
	}

	return nil
}

func (field *UAVObjectFieldDefinition) interfaceToUAVTalk(writer *bytes.Buffer, value interface{}) error {
	if field.Elements > 1 {
		valueArray, ok := value.([]interface{})

		if ok == false {
			return errors.New("Value should be a slice for fields with Elements > 1")
		}

		for _, value := range valueArray {
			if err := field.writeToUAVTalk(writer, value); err != nil {
				return err
			}
		}
	} else {
		if err := field.writeToUAVTalk(writer, value); err != nil {
			return err
		}
	}
	return nil
}

/**
 * UAVObjectDefinition
 */

func (uavdef *UAVObjectDefinition) mapToUAVTalk(data map[string]interface{}) ([]byte, error) {
	writer := new(bytes.Buffer)
	for _, field := range uavdef.Fields {
		if err := field.interfaceToUAVTalk(writer, data[field.Name]); err != nil {
			return nil, err
		}
	}

	return writer.Bytes(), nil
}
