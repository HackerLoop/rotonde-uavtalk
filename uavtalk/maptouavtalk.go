package uavtalk

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

func valueForEnumString(field *FieldDefinition, option string) (uint8, error) {
	for val, opt := range field.Options {
		if opt == option {
			return uint8(val), nil
		}
	}
	return 0, fmt.Errorf("%s enum option not found", option)
}

func writeToUAVTalk(field *FieldDefinition, writer *bytes.Buffer, value interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()
	typeInfo := field.FieldTypeInfo
	var result interface{}
	switch typeInfo.Name {
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
		if result, err = valueForEnumString(field, value.(string)); err != nil {
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

func interfaceToUAVTalk(field *FieldDefinition, writer *bytes.Buffer, value interface{}) error {
	if field.Elements > 1 && len(field.ElementNames) == 0 {
		valueArray, ok := value.([]interface{})

		if ok == false {
			return fmt.Errorf("Value for %s should be a map of fields with Elements > 1", field.Name)
		}

		for _, value := range valueArray {
			if err := writeToUAVTalk(field, writer, value); err != nil {
				return err
			}
		}
	} else if field.Elements > 1 && len(field.ElementNames) > 0 {
		valueMap, ok := value.(map[string]interface{})

		if ok == false {
			return fmt.Errorf("Value for %s should be a map of fields with Elements > 1", field.Name)
		}

		for _, name := range field.ElementNames {
			value := valueMap[name]
			if err := writeToUAVTalk(field, writer, value); err != nil {
				return err
			}
		}
	} else {
		if err := writeToUAVTalk(field, writer, value); err != nil {
			return err
		}
	}
	return nil
}

func mapToUAVTalk(uavdef *Definition, data map[string]interface{}) ([]byte, error) {
	writer := new(bytes.Buffer)
	for _, field := range uavdef.Fields {
		if err := interfaceToUAVTalk(field, writer, data[field.Name]); err != nil {
			return nil, err
		}
	}

	return writer.Bytes(), nil
}
