package main

import (
	"bytes"
	"fmt"
	"log"

	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
)

type Hash uint32

func (hash *Hash) updateHashWithInt(value uint32) {
	*hash = Hash(uint32(*hash) ^ ((uint32(*hash) << 5) + (uint32(*hash) >> 2) + value))
}

func (hash *Hash) updateHashWithBool(value bool) {
	if value {
		hash.updateHashWithInt(1)
	} else {
		hash.updateHashWithInt(0)
	}
}

func (hash *Hash) updateHashWithString(value string) {
	latin1, err := toISO88591(value)
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range []byte(latin1) {
		hash.updateHashWithInt(uint32(b))
	}
}

func toISO88591(utf8 string) (string, error) {
	buf := new(bytes.Buffer)
	w, err := charset.NewWriter("latin1", buf)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(w, utf8)
	w.Close()
	return buf.String(), nil
}

func (uavdef *UAVObjectDefinition) calculateId() {
	hash := new(Hash)

	//fmt.Println(uavdef.Name)
	//fmt.Println(*hash)
	hash.updateHashWithString(uavdef.Name)
	//fmt.Println(*hash)
	hash.updateHashWithBool(uavdef.Settings)
	//fmt.Println(*hash)
	hash.updateHashWithBool(uavdef.SingleInstance)
	//fmt.Println(*hash)

	for _, field := range uavdef.Fields {
		//fmt.Println(field.Name)
		hash.updateHashWithString(field.Name)
		//fmt.Println(*hash)
		hash.updateHashWithInt(uint32(field.Elements))
		//fmt.Println(field.Elements)
		//fmt.Println(*hash)
		hash.updateHashWithInt(uint32(field.fieldTypeInfo.index))
		//fmt.Println(field.Type)
		//fmt.Println(*hash)

		if field.Type == "enum" {
			//fmt.Println("enum")
			for _, option := range field.Options {
				hash.updateHashWithString(option)
				//fmt.Println(*hash)
			}
		}
	}

	uavdef.ObjectID = uint32(*hash) & 0xFFFFFFFE
}
