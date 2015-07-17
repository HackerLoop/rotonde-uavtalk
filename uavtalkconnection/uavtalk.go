package uavtalkconnection

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/common"
	"github.com/openflylab/bridge/dispatcher"
	"github.com/openflylab/bridge/utils"
)

var definitions common.Definitions

// newDefinitions loads all xml files from a directory
func newDefinitions(dir string) (common.Definitions, error) {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	definitions := make([]*common.Definition, 0, 150)
	for _, fileInfo := range fileInfos {
		filePath := fmt.Sprintf("%s%s", dir, fileInfo.Name())
		definition, err := newDefinition(filePath)
		if err != nil {
			log.Fatal(err)
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

// NewDefinition create an Definition from an xml file.
func newDefinition(filePath string) (*common.Definition, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(file)

	var content = &struct {
		Definition *common.Definition `xml:"object"`
	}{}
	decoder.Decode(content)

	definition := content.Definition

	// fields post process
	for _, field := range definition.Fields {
		if len(field.CloneOf) != 0 {
			continue
		}

		if field.Elements == 0 {
			field.Elements = 1
		}

		if len(field.ElementNamesAttr) > 0 {
			field.ElementNames = strings.Split(field.ElementNamesAttr, ",")
			field.Elements = len(field.ElementNames)
		} else if len(field.ElementNames) > 0 {
			field.Elements = len(field.ElementNames)
		}

		if len(field.OptionsAttr) > 0 {
			field.Options = strings.Split(field.OptionsAttr, ",")
		}

		field.FieldTypeInfo, err = common.TypeInfos.FieldTypeForString(field.Type)
		if err != nil {
			return nil, err
		}
	}

	// create clones
	for _, field := range definition.Fields {
		if len(field.CloneOf) != 0 {
			clonedField, err := definition.Fields.FieldForName(field.CloneOf)
			if err != nil {
				return nil, err
			}
			name, cloneOf := field.Name, field.CloneOf
			*field = *clonedField
			field.Name, field.CloneOf = name, cloneOf
		}
	}

	sort.Stable(definition.Fields)

	calculateID(definition)

	return definition, nil
}

// TODO: refactor for better value reading (encoding/binary ?)
// See uavtalk.cpp state machine pattern in GCS

const versionMask = 0x20
const shortHeaderLength = 8

const maxHIDFrameSize = 64

const objectCmd = 0
const objectRequest = 1
const objectCmdWithAck = 2
const objectAck = 3
const objectNack = 4

// Packet data from/to the flight controller
type Packet struct {
	definition *common.Definition
	cmd        uint8
	length     uint16
	instanceID uint16
	data       map[string]interface{}
}

func (buffer *Packet) toBinary() ([]byte, error) {
	writer := new(bytes.Buffer)

	if err := binary.Write(writer, binary.LittleEndian, uint8(0x3c)); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, buffer.cmd|versionMask); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, buffer.length); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, buffer.definition.ObjectID); err != nil {
		return nil, err
	}

	if buffer.definition.SingleInstance == false {
		if err := binary.Write(writer, binary.LittleEndian, buffer.instanceID); err != nil {
			return nil, err
		}
	}

	if buffer.cmd == objectCmd || buffer.cmd == objectCmdWithAck {
		data, err := mapToUAVTalk(buffer.definition, buffer.data)
		if err != nil {
			return nil, err
		}

		if err := binary.Write(writer, binary.LittleEndian, data); err != nil {
			return nil, err
		}
	}

	cks := computeCrc8(0, writer.Bytes())
	if err := binary.Write(writer, binary.LittleEndian, cks); err != nil {
		return nil, err
	}

	return writer.Bytes(), nil
}

func byteArrayToInt32(b []byte) uint32 {
	if len(b) != 4 {
		panic("byteArrayToInt32 requires at least 4 bytes")
	}

	return (uint32(b[3]) << 24) | (uint32(b[2]) << 16) | (uint32(b[1]) << 8) | (uint32(b[0]))
}

func byteArrayToInt16(b []byte) uint16 {
	if len(b) != 2 {
		panic("byteArrayToInt16 requires at least 2 bytes")
	}

	return (uint16(b[1]) << 8) | (uint16(b[0]))
}

func bufferComplete(buffer []byte) (bool, int, int, error) {
	offset := -1
	for i := 0; i < len(buffer); i++ {
		if buffer[i] == 0x3c {
			offset = i
			break
		}
	}

	if offset < 0 {
		return false, 0, 0, nil
	}

	length := byteArrayToInt16(buffer[offset+2 : offset+4])

	if int(length)+1 > len(buffer)-offset {
		return false, 0, 0, nil
	}

	cks := buffer[offset+int(length)]

	if cks != computeCrc8(0, buffer[offset:offset+int(length)]) {
		return false, offset, offset + int(length) + 1, fmt.Errorf("Wrong crc8 !!!!")
	}

	return true, offset, offset + int(length) + 1, nil
}

func newPacketFromBinary(binaryPacket []byte) (*Packet, error) {
	headerSize := shortHeaderLength
	buffer := Packet{}

	buffer.cmd = binaryPacket[1] ^ versionMask
	buffer.length = byteArrayToInt16(binaryPacket[2:4])
	objectID := byteArrayToInt32(binaryPacket[4:8])

	var err error
	buffer.definition, err = definitions.GetDefinitionForObjectID(objectID)
	if err != nil {
		return nil, err
	}
	if buffer.definition.SingleInstance == false {
		buffer.instanceID = byteArrayToInt16(binaryPacket[8:10])
		headerSize += 2
	}

	binaryData := binaryPacket[headerSize : len(binaryPacket)-1]

	if buffer.cmd == objectCmd || buffer.cmd == objectCmdWithAck {
		buffer.data, err = uAVTalkToMap(buffer.definition, binaryData)
		if err != nil {
			return nil, err
		}
	} else {
		buffer.data = map[string]interface{}{}
	}

	return &buffer, nil
}

func newPacket(definition *common.Definition, cmd uint8, instanceID uint16, data map[string]interface{}) *Packet {
	buffer := Packet{}
	buffer.definition = definition
	buffer.cmd = cmd
	buffer.instanceID = instanceID

	var fieldsLength int
	if cmd == objectCmd || cmd == objectCmdWithAck {
		fieldsLength = definition.Fields.ByteLength()
	}

	if buffer.definition.SingleInstance == false {
		buffer.length = uint16(shortHeaderLength + fieldsLength + 2)
	} else {
		buffer.length = uint16(shortHeaderLength + fieldsLength)
	}
	buffer.data = data
	return &buffer
}

// Start starts the HID driver
func Start(d *dispatcher.Dispatcher, definitionsDir string) {
	defs, err := newDefinitions(definitionsDir)
	if err != nil {
		log.Fatal(err)
	}
	definitions = defs

	log.Infof("%d xml files loaded\n", len(definitions))
	for _, definition := range definitions {
		log.Infof("Name: %s ObjectID: %d", definition.Name, definition.ObjectID)
	}

	sh := newStateHolder(d)

	link, err := newUSBLink() //newTCPLink()
	if err != nil {
		log.Fatal(err)
	}
	defer link.Close()

	// From USB
	go func() {
		packet := make([]byte, maxHIDFrameSize)
		buffer := make([]byte, 0, 4096)
		for {
			n, err := link.Read(packet)
			if err != nil {
				log.Fatal(err)
			}
			if n == 0 {
				continue
			}
			//log.Info("received:")
			//utils.PrintHex(packet, len(packet[0:n]))

			buffer = append(buffer, packet[0:n]...)
			//log.Info(len(buffer))
			//log.Info("buffer:")
			//utils.PrintHex(buffer, len(buffer))

			for {
				ok, from, to, err := bufferComplete(buffer)
				if err == nil {
					if ok != true {
						break
					}
					//log.Info("buffer complete:")
					//utils.PrintHex(buffer[from:to], to-from)

					if uavTalkObject, err := newPacketFromBinary(buffer[from:to]); err == nil {
						sh.outChan <- *uavTalkObject
					} else {
						log.Warning(err)
					}
				} else {
					log.Warning(err)
					utils.PrintHex(buffer[from:to], to-from)
				}
				copy(buffer, buffer[to:]) // baaaaah !! ring packet to the rescue ?
				buffer = buffer[0 : len(buffer)-to]
				//log.Info("buffer:")
				//utils.PrintHex(buffer, len(buffer))
			}
		}
	}()

	// To Controller
	go func() {
		for {
			buffer := <-sh.inChan

			binaryPacket, err := buffer.toBinary()
			if err != nil {
				log.Println(err)
				continue
			}

			//log.Info("sending")
			//utils.PrintHex(binaryPacket, len(binaryPacket))

			_, err = link.Write(binaryPacket)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	select {}
}