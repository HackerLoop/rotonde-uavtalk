package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
)

type TypeIndex []string

var types = TypeIndex{"int8", "int16", "int32", "uint8", "uint16", "uint32", "float", "enum"}

func (t TypeIndex) indexForString(ts string) int {
	for i, typeIndexStr := range types {
		if typeIndexStr == ts {
			return i
		}
	}
	return -1
}

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

	hash.updateHashWithString(uavdef.Name)
	hash.updateHashWithBool(uavdef.Settings)
	hash.updateHashWithBool(uavdef.SingleInstance)

	for _, field := range uavdef.Fields {
		hash.updateHashWithString(field.Name)
		hash.updateHashWithInt(uint32(field.Elements))
		hash.updateHashWithInt(uint32(types.indexForString(field.Type)))

		if field.Type == "enum" {
			options := strings.Split(field.Options, ",")
			for _, option := range options {
				hash.updateHashWithString(option)
			}
		}
	}

	uavdef.ObjectID = uint32(*hash) & 0xFFFFFFFE
}
