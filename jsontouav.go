package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

/**
 * UAVObjectFieldDefinition
 */

func (field *UAVObjectFieldDefinition) valueForEnumString(option string) (uint8, error) {
	for val, opt := range field.Options {
		if opt == option {
			return uint8(val), nil
		}
	}
	return 0, errors.New(fmt.Sprintf("%s enum option not found", option))
}

func (field *UAVObjectFieldDefinition) writeToUAVTalk(writer *bytes.Buffer, value interface{}) error {
	typeInfo := field.fieldTypeInfo
	var result interface{}
	switch typeInfo.name {
	case "int8":
		result = uint8(value.(float64))
	case "int16":
		result = int16(value.(float64))
	case "int32":
		result = int32(value.(float64))
	case "uint8":
		result = uint8(value.(float64))
	case "uint16":
		result = uint16(value.(float64))
	case "uint32":
		result = uint32(value.(float64))
	case "float":
		result = float32(value.(float64))
	case "enum":
		var err error
		if result, err = field.valueForEnumString(value.(string)); err != nil {
			return err
		}
	}
	if result == nil {
		return errors.New("Could not read from typeInfo.")
	}
	if err := binary.Write(writer, binary.LittleEndian, result); err != nil {
		return err
	}

	return nil
}

func (field *UAVObjectFieldDefinition) interfaceToUAVTalk(writer *bytes.Buffer, value interface{}) error {
	if field.Elements > 1 && len(field.ElementNames) == 0 {
		valueArray, ok := value.([]interface{})

		if ok == false {
			return errors.New("Value should be a slice for fields with Elements > 1")
		}

		for _, value := range valueArray {
			if err := field.writeToUAVTalk(writer, value); err != nil {
				return err
			}
		}
	} else if field.Elements > 1 && len(field.ElementNames) > 0 {
		valueMap, ok := value.(map[string]interface{})

		if ok == false {
			return errors.New("Value should be a map for fields with Elements > 1")
		}

		for _, name := range field.ElementNames {
			value := valueMap[name]
			fmt.Println(name)
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
