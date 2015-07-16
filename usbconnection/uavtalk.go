package usbconnection

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/GeertJohan/go.hid"
	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/dispatcher"
	"github.com/openflylab/bridge/uavobject"
	"github.com/openflylab/bridge/utils"
)

var definitions uavobject.Definitions

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
	definition *uavobject.Definition
	cmd        uint8
	length     uint16
	instanceID uint16
	data       map[string]interface{}
}

func (packet *Packet) toBinary() ([]byte, error) {
	writer := new(bytes.Buffer)

	if err := binary.Write(writer, binary.LittleEndian, uint8(0x3c)); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, packet.cmd|versionMask); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, packet.length); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, packet.definition.ObjectID); err != nil {
		return nil, err
	}

	if packet.definition.SingleInstance == false {
		if err := binary.Write(writer, binary.LittleEndian, packet.instanceID); err != nil {
			return nil, err
		}
	}

	if packet.cmd == objectCmd || packet.cmd == objectCmdWithAck {
		data, err := mapToUAVTalk(packet.definition, packet.data)
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

func packetComplete(packet []byte) (bool, int, int, error) {
	offset := -1
	for i := 0; i < len(packet); i++ {
		if packet[i] == 0x3c {
			offset = i
			break
		}
	}

	if offset < 0 {
		return false, 0, 0, nil
	}

	length := byteArrayToInt16(packet[offset+2 : offset+4])

	if int(length)+1 > len(packet)-offset {
		return false, 0, 0, nil
	}

	cks := packet[offset+int(length)]

	if cks != computeCrc8(0, packet[offset:offset+int(length)]) {
		return false, offset, offset + int(length) + 1, fmt.Errorf("Wrong crc8 !!!!")
	}

	return true, offset, offset + int(length) + 1, nil
}

func newPacketFromBinary(binaryPacket []byte) (*Packet, error) {
	headerSize := shortHeaderLength
	packet := Packet{}

	packet.cmd = binaryPacket[1] ^ versionMask
	packet.length = byteArrayToInt16(binaryPacket[2:4])
	objectID := byteArrayToInt32(binaryPacket[4:8])

	var err error
	packet.definition, err = definitions.GetDefinitionForObjectID(objectID)
	if err != nil {
		return nil, err
	}
	if packet.definition.SingleInstance == false {
		packet.instanceID = byteArrayToInt16(binaryPacket[8:10])
		headerSize += 2
	}

	binaryData := binaryPacket[headerSize : len(binaryPacket)-1]

	if packet.cmd == objectCmd || packet.cmd == objectCmdWithAck {
		packet.data, err = uAVTalkToMap(packet.definition, binaryData)
		if err != nil {
			return nil, err
		}
	} else {
		packet.data = map[string]interface{}{}
	}

	return &packet, nil
}

func newPacket(definition *uavobject.Definition, cmd uint8, instanceID uint16, data map[string]interface{}) *Packet {
	packet := Packet{}
	packet.definition = definition
	packet.cmd = cmd
	packet.instanceID = instanceID

	var fieldsLength int
	if cmd == objectCmd || cmd == objectCmdWithAck {
		fieldsLength = definition.Fields.ByteLength()
	}

	if packet.definition.SingleInstance == false {
		packet.length = uint16(shortHeaderLength + fieldsLength + 2)
	} else {
		packet.length = uint16(shortHeaderLength + fieldsLength)
	}
	packet.data = data
	return &packet
}

// Start starts the HID driver
func Start(d *dispatcher.Dispatcher, definitionsDir string) {
	defs, err := uavobject.NewDefinitions(definitionsDir)
	if err != nil {
		log.Fatal(err)
	}
	definitions = defs

	log.Infof("%d xml files loaded\n", len(definitions))
	for _, definition := range definitions {
		log.Infof("Name: %s ObjectID: %d", definition.Name, definition.ObjectID)
	}

	sh := newStateHolder(d)

	cc, err := hid.Open(0x20a0, 0x41d0, "")
	if err != nil {
		log.Fatal(err)
	}
	defer cc.Close()
	defer log.Println("Closing HID")

	/*c := &serial.Config{Name: "/dev/cu.usbmodem1421", Baud: 57600}
	cc, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}*/

	// From USB
	go func() {
		buffer := make([]byte, maxHIDFrameSize)
		packet := make([]byte, 0, 4096)
		for {
			n, err := cc.ReadTimeout(buffer, 50)
			if err != nil {
				log.Fatal(err)
			}
			if n == 0 {
				continue
			}
			//log.Info("received:")
			//utils.PrintHex(buffer, int(2+buffer[1]))

			packet = append(packet, buffer[2:2+buffer[1]]...)
			//log.Info(len(packet))
			//log.Info("packet:")
			//utils.PrintHex(packet, len(packet))

			for {
				ok, from, to, err := packetComplete(packet)
				if err == nil {
					if ok != true {
						break
					}
					//log.Info("packet complete:")
					//utils.PrintHex(packet[from:to], to-from)

					if uavTalkObject, err := newPacketFromBinary(packet[from:to]); err == nil {
						sh.outChan <- *uavTalkObject
					} else {
						log.Warning(err)
					}
				} else {
					log.Warning(err)
					utils.PrintHex(packet[from:to], to-from)
				}
				copy(packet, packet[to:]) // baaaaah !! ring buffer to the rescue ?
				packet = packet[0 : len(packet)-to]
			}
		}
	}()

	// To USB
	go func() {
		fixedLengthWriteBuffer := make([]byte, maxHIDFrameSize)
		for {
			packet := <-sh.inChan

			binaryPacket, err := packet.toBinary()
			if err != nil {
				log.Println(err)
				continue
			}

			//log.Info("sending")
			//utils.PrintHex(binaryPacket, len(binaryPacket))

			currentOffset := 0
			for currentOffset < len(binaryPacket) {
				toWriteLength := len(binaryPacket) - currentOffset
				if toWriteLength > maxHIDFrameSize-2 {
					toWriteLength = maxHIDFrameSize - 2
				}
				copy(fixedLengthWriteBuffer, append([]byte{0x02, byte(toWriteLength)}, binaryPacket[currentOffset:currentOffset+toWriteLength]...))
				log.Info("sending")
				utils.PrintHex(fixedLengthWriteBuffer, len(fixedLengthWriteBuffer))
				_, err := cc.Write(fixedLengthWriteBuffer)
				if err != nil {
					log.Fatal(err)
				}
				currentOffset += toWriteLength
			}
		}
	}()

	select {}
}
