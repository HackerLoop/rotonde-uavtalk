package uavtalk

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
)

var AllDefinitions Definitions
var maxUAVObjectLength int

// TODO: refactor for better value reading (encoding/binary ?)
// See uavtalk.cpp state machine pattern in GCS
// see parsing in rotonde HID

const versionMask = 0x20
const shortHeaderLength = 8

const MaxHIDFrameSize = 64

const ObjectCmd = 0
const ObjectRequest = 1
const ObjectCmdWithAck = 2
const ObjectAck = 3
const ObjectNack = 4

// Packet data from/to the flight controller
type Packet struct {
	Definition *Definition
	Cmd        uint8
	Length     uint16
	InstanceID uint16
	Data       map[string]interface{}
}

func (packet *Packet) toBinary() ([]byte, error) {
	writer := new(bytes.Buffer)

	if err := binary.Write(writer, binary.LittleEndian, uint8(0x3c)); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, packet.Cmd|versionMask); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, packet.Length); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, packet.Definition.ObjectID); err != nil {
		return nil, err
	}

	if packet.Definition.SingleInstance == false {
		if err := binary.Write(writer, binary.LittleEndian, packet.InstanceID); err != nil {
			return nil, err
		}
	}

	if packet.Cmd == ObjectCmd || packet.Cmd == ObjectCmdWithAck {
		data, err := mapToUAVTalk(packet.Definition, packet.Data)
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

func packetComplete(buffer []byte) (bool, int, int, error) {
	start := 0
	for {
		offset := -1
		for i := start; i < len(buffer)-shortHeaderLength+1; i++ {
			if buffer[i] == 0x3c {
				offset = i
				break
			}
		}

		if offset < 0 {
			return false, 0, 0, nil
		}

		length := byteArrayToInt16(buffer[offset+2 : offset+4])

		if int(length) > maxUAVObjectLength+shortHeaderLength+2 {
			start = offset + 1
			continue
		}

		if int(length)+1 > len(buffer)-offset {
			return false, 0, 0, nil
		}

		cks := buffer[offset+int(length)]

		if cks != computeCrc8(0, buffer[offset:offset+int(length)]) {
			return false, offset, offset + int(length) + 1, fmt.Errorf("Wrong crc8")
		}

		return true, offset, offset + int(length) + 1, nil
	}
}

func newPacketFromBinary(binaryPacket []byte) (*Packet, error) {
	headerSize := shortHeaderLength
	buffer := Packet{}

	buffer.Cmd = binaryPacket[1] ^ versionMask
	buffer.Length = byteArrayToInt16(binaryPacket[2:4])
	objectID := byteArrayToInt32(binaryPacket[4:8])

	var err error
	buffer.Definition, err = AllDefinitions.GetDefinitionForObjectID(objectID)
	if err != nil {
		return nil, err
	}
	if buffer.Definition.SingleInstance == false {
		buffer.InstanceID = byteArrayToInt16(binaryPacket[8:10])
		headerSize += 2
	}

	binaryData := binaryPacket[headerSize : len(binaryPacket)-1]

	if buffer.Cmd == ObjectCmd || buffer.Cmd == ObjectCmdWithAck {
		buffer.Data, err = uAVTalkToMap(buffer.Definition, binaryData)
		if err != nil {
			return nil, err
		}
	} else {
		buffer.Data = map[string]interface{}{}
	}

	return &buffer, nil
}

func NewPacket(definition *Definition, cmd uint8, instanceID uint16, data map[string]interface{}) *Packet {
	buffer := Packet{}
	buffer.Definition = definition
	buffer.Cmd = cmd
	buffer.InstanceID = instanceID

	var fieldsLength int
	if cmd == ObjectCmd || cmd == ObjectCmdWithAck {
		fieldsLength = definition.Fields.ByteLength()
	}

	if buffer.Definition.SingleInstance == false {
		buffer.Length = uint16(shortHeaderLength + fieldsLength + 2)
	} else {
		buffer.Length = uint16(shortHeaderLength + fieldsLength)
	}
	buffer.Data = data
	return &buffer
}

func LoadDefinitions(definitionsDir string) {
	defs, err := newDefinitions(definitionsDir)
	if err != nil {
		log.Fatal(err)
	}
	AllDefinitions = defs
}

// Start starts the UAVTalk connection to dispatcher
func Start(inChan chan Packet, outChan chan Packet) {
	for _, definition := range AllDefinitions {
		tmp := definition.Fields.ByteLength()
		tmp += shortHeaderLength
		if definition.SingleInstance == false {
			tmp += 2
		}
		if tmp > maxUAVObjectLength {
			maxUAVObjectLength = tmp
		}
	}

	log.Infof("%d xml files loaded, maxUAVObjectLength: %d", len(AllDefinitions), maxUAVObjectLength)

	for {
		start(inChan, outChan)
	}
}

func recoverChanClosed(dir string) {
	if e := recover(); e != nil {
		log.Info("Recovered in start, direction: ", dir, e)
	}
}

func start(inChan chan Packet, outChan chan Packet) {
	var link Linker
	var err error
	for {
		link, err = NewTCPLink() // newUSBLink() ou newTCPLink()
		if err != nil {
			log.Warning(err)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	linkError := make(chan error)
	defer close(linkError)
	defer link.Close()
	// From Controller
	go func() {
		defer recoverChanClosed("Out")
		packet := make([]byte, MaxHIDFrameSize)
		buffer := make([]byte, 0, 4096)
		for {
			n, err := link.Read(packet)
			if err != nil {
				linkError <- err
				return
			}
			if n == 0 {
				continue
			}

			buffer = append(buffer, packet[0:n]...)

			for {
				ok, from, to, err := packetComplete(buffer)
				if err == nil {
					if ok != true {
						break
					}

					if uavTalkObject, err := newPacketFromBinary(buffer[from:to]); err == nil {
						outChan <- *uavTalkObject
					} else {
						log.Warning(err)
						PrintHex(buffer[from:to], to-from)
					}
				} else {
					// the packet is complete but its integrity is seriously questionned,
					// we go through so we can strip it from buffer
					log.Warning(err)
					PrintHex(buffer[from:to], to-from)
				}
				copy(buffer, buffer[to:])
				buffer = buffer[0 : len(buffer)-to]
			}
		}
	}()

	// To Controller
	go func() {
		defer recoverChanClosed("In")
		for {
			var binaryPacket []byte
			select {
			case packet := <-inChan:
				binaryPacket, err = packet.toBinary()
				if err != nil {
					log.Warning(err)
					continue
				}
			}

			_, err = link.Write(binaryPacket)
			if err != nil {
				linkError <- err
				return
			}
		}
	}()

	err = <-linkError
	log.Warning(err)
}

// newDefinitions loads all xml files from a directory
func newDefinitions(dir string) (Definitions, error) {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	AllDefinitions := make([]*Definition, 0, 150)
	for _, fileInfo := range fileInfos {
		filePath := fmt.Sprintf("%s%s", dir, fileInfo.Name())
		definition, err := newDefinition(filePath)
		if err != nil {
			log.Fatal(err)
		}
		_, err = NewMetaDefinition(definition)
		if err != nil {
			log.Fatal(err)
		}
		AllDefinitions = append(AllDefinitions, definition, definition.Meta)
	}
	return AllDefinitions, nil
}

// NewDefinition create a Definition from an xml file.
func newDefinition(filePath string) (*Definition, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(file)

	var content = &struct {
		Definition *Definition `xml:"object"`
	}{}
	decoder.Decode(content)

	definition := content.Definition
	if err := definition.FinishSetup(); err != nil {
		return nil, err
	}

	calculateID(definition)

	return definition, nil
}
