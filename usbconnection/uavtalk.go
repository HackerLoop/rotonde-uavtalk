package usbconnection

import (
	"bytes"
	"encoding/binary"

	"github.com/GeertJohan/go.hid"
	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/dispatcher"
	"github.com/openflylab/bridge/uavobject"
)

var definitions uavobject.Definitions

// TODO: refactor for better value reading (encoding/binary)

const versionMask = 0x20
const shortHeaderLength = 8

// Packet data from/to the flight controller
type Packet struct {
	cmd        uint8
	length     uint16
	objectID   uint32
	instanceID uint16
	data       []byte
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

	if err := binary.Write(writer, binary.LittleEndian, packet.objectID); err != nil {
		return nil, err
	}

	if unique, _ := definitions.IsUniqueInstanceForObjectID(packet.objectID); !unique {
		if err := binary.Write(writer, binary.LittleEndian, packet.instanceID); err != nil {
			return nil, err
		}
	}

	if packet.cmd == 0 || packet.cmd == 2 {
		if err := binary.Write(writer, binary.LittleEndian, packet.data); err != nil {
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

// TODO: find a better way to check if a packet is complete
func packetComplete(packet []byte) (bool, int, int) {
	headerSize := shortHeaderLength
	offset := -1
	for i := 0; i < len(packet); i++ {
		if packet[i] == 0x3c {
			offset = i
			break
		}
	}

	if offset < 0 {
		return false, 0, 0
	}

	// TODO: refac double call with newUAVTalkObjectFromBinary
	objectID := byteArrayToInt32(packet[4:8])
	if unique, _ := definitions.IsUniqueInstanceForObjectID(objectID); !unique {
		headerSize += 2
	}

	frame := packet[offset:]

	if len(frame) < headerSize+1 {
		return false, 0, 0
	}

	length := byteArrayToInt16(frame[2:4])

	if int(length)+1 > len(frame) {
		return false, 0, 0
	}

	cks := frame[length]

	if cks != computeCrc8(0, packet[0:length]) {
		return false, 0, 0
	}

	log.Info(offset)

	return true, offset, offset + int(length) + 1
}

func newPacketFromBinary(binaryPacket []byte) (*Packet, error) {
	headerSize := shortHeaderLength
	packet := &Packet{}

	packet.cmd = binaryPacket[1] ^ versionMask
	packet.length = byteArrayToInt16(binaryPacket[2:4])
	packet.objectID = byteArrayToInt32(binaryPacket[4:8])

	if unique, _ := definitions.IsUniqueInstanceForObjectID(packet.objectID); !unique {
		packet.instanceID = byteArrayToInt16(binaryPacket[8:10])
		headerSize += 2
	}

	packet.data = make([]byte, int(packet.length)-headerSize)
	copy(packet.data, binaryPacket[headerSize:len(binaryPacket)-1])

	return packet, nil
}

func newPacket(cmd uint8, objectID uint32, instanceID uint16, data []byte) (*Packet, error) {
	packet := new(Packet)
	packet.cmd = cmd
	packet.objectID = objectID
	packet.instanceID = instanceID
	if unique, _ := definitions.IsUniqueInstanceForObjectID(packet.objectID); !unique {
		packet.length = uint16(shortHeaderLength + len(data) + 2)
	} else {
		packet.length = uint16(shortHeaderLength + len(data))
	}
	packet.data = data
	return packet, nil
}

// Start starts the HID driver, and connects it to the dispatcher
func Start(d *dispatcher.Dispatcher, definitionsDir string) {
	defs, err := uavobject.NewDefinitions(definitionsDir)
	if err != nil {
		log.Fatal(err)
	}
	definitions = defs

	log.Printf("%d xml files loaded\n", len(definitions))

	c := dispatcher.NewConnection()
	d.AddConnection(c)

	cc, err := hid.Open(0x20a0, 0x41d0, "")
	if err != nil {
		log.Fatal(err)
	}
	defer cc.Close()
	defer log.Println("Closing HID")

	log.Println("Starting HID")

	// From usb
	go func() {
		buffer := make([]byte, 64)
		packet := make([]byte, 0, 4096)
		for {
			_, err := cc.Read(buffer)
			if err != nil {
				log.Fatal(err)
			}

			packet = append(packet[len(packet):], buffer[2:2+buffer[1]]...)

			for {
				ok, from, to := packetComplete(packet)
				if ok != true {
					break
				}
				if uavTalkObject, err := newPacketFromBinary(packet[from:to]); err == nil {
					definition, err := definitions.GetDefinitionForObjectID(uavTalkObject.objectID)
					if err != nil {
						log.Warning(err)
						continue
					}

					data, err := uAVTalkToMap(definition, uavTalkObject.data)
					if err != nil {
						log.Warning(err)
						continue
					}

					update := dispatcher.Update{uavTalkObject.objectID, uavTalkObject.instanceID, data}
					c.OutChan <- update
				} else {
					log.Println(err)
				}
				copy(packet, packet[from:]) // baaaaah !! ring buffer to the rescue ?
				packet = packet[0 : len(packet)-to]
			}
		}
	}()

	// From dispatcher
	go func() {
		for {
			dispatcherMsg := <-c.InChan
			switch dispatcherPacket := dispatcherMsg.(type) {
			case dispatcher.Update:
				definition, err := definitions.GetDefinitionForObjectID(dispatcherPacket.ObjectID)
				if err != nil {
					log.Warning(err)
					continue
				}

				data, err := mapToUAVTalk(definition, dispatcherPacket.Data)
				if err != nil {
					log.Warning(err)
					continue
				}

				var cmd uint8
				if definition.TelemetryFlight.Acked {
					cmd = 2
				}
				packet, err := newPacket(cmd, dispatcherPacket.ObjectID, dispatcherPacket.InstanceID, data)
				if err != nil {
					log.Warning(err)
					continue
				}

				binaryObj, err := packet.toBinary()
				if err != nil {
					log.Println(err)
					continue
				}
				binaryObj = append([]byte{0x01, byte(len(binaryObj))}, binaryObj...)
				//printHex(binaryObj, len(binaryObj))

				_, err = cc.Write(binaryObj)
				if err != nil {
					log.Fatal(err)
				}
			}
			//log.Println("Bytes sent", n)
		}
	}()

	select {}
}
