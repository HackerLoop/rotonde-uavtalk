package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/GeertJohan/go.hid"
)

// TODO: refactor for better value reading (encoding/binary)

const VER_MASK = 0x20
const SHORT_HEADER_LENGTH = 8

type UAVTalkObject struct {
	cmd        uint8
	length     uint16
	objectId   uint32
	instanceId uint16
	data       []byte
}

func (uavTalkObject *UAVTalkObject) toBinary() ([]byte, error) {
	writer := new(bytes.Buffer)

	if err := binary.Write(writer, binary.LittleEndian, uint8(0x3c)); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, uavTalkObject.cmd|VER_MASK); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, uavTalkObject.length); err != nil {
		return nil, err
	}

	if err := binary.Write(writer, binary.LittleEndian, uavTalkObject.objectId); err != nil {
		return nil, err
	}

	if unique, _ := isUniqueInstanceForObjectID(uavTalkObject.objectId); !unique {
		if err := binary.Write(writer, binary.LittleEndian, uavTalkObject.instanceId); err != nil {
			return nil, err
		}
	}

	if uavTalkObject.cmd == 0 || uavTalkObject.cmd == 2 {
		if err := binary.Write(writer, binary.LittleEndian, uavTalkObject.data); err != nil {
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
	headerSize := SHORT_HEADER_LENGTH
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
	objectId := byteArrayToInt32(packet[4:8])
	if unique, _ := isUniqueInstanceForObjectID(objectId); !unique {
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

	return true, offset, offset + int(length) + 1
}

func newUAVTalkObjectFromBinary(packet []byte) (*UAVTalkObject, error) {
	headerSize := SHORT_HEADER_LENGTH
	uavTalkObject := &UAVTalkObject{}

	uavTalkObject.cmd = packet[1] ^ VER_MASK
	uavTalkObject.length = byteArrayToInt16(packet[2:4])
	uavTalkObject.objectId = byteArrayToInt32(packet[4:8])

	if unique, _ := isUniqueInstanceForObjectID(uavTalkObject.objectId); !unique {
		uavTalkObject.instanceId = byteArrayToInt16(packet[8:10])
		headerSize += 2
	}

	uavTalkObject.data = make([]byte, int(uavTalkObject.length)-headerSize)
	copy(uavTalkObject.data, packet[headerSize:len(packet)-1])

	return uavTalkObject, nil
}

func newUAVTalkObject(cmd uint8, objectId uint32, instanceId uint16, data []byte) (*UAVTalkObject, error) {
	uavTalkObject := new(UAVTalkObject)
	uavTalkObject.cmd = cmd
	uavTalkObject.objectId = objectId
	uavTalkObject.instanceId = instanceId
	if unique, _ := isUniqueInstanceForObjectID(uavTalkObject.objectId); !unique {
		uavTalkObject.length = uint16(SHORT_HEADER_LENGTH + len(data) + 2)
	} else {
		uavTalkObject.length = uint16(SHORT_HEADER_LENGTH + len(data))
	}
	uavTalkObject.data = data
	return uavTalkObject, nil
}

func printHex(buffer []byte, n int) {
	fmt.Println("start packet:")
	for i := 0; i < n; i++ {
		if i > 0 {
			fmt.Print(":")
		}
		fmt.Printf("%.02x", buffer[i])
	}
	fmt.Println("\nend packet")
}

func startHID(stopChan chan bool, uavChan chan *UAVTalkObject, jsonChan chan *UAVTalkObject) {
	cc, err := hid.Open(0x20a0, 0x41d0, "")
	if err != nil {
		log.Fatal(err)
	}
	defer cc.Close()
	defer log.Println("Closing HID")

	log.Println("Starting HID")

	uavStopChan := make(chan bool)
	// uav goroutine
	go func() {
		buffer := make([]byte, 64)
		packet := make([]byte, 0, 4096)
		for {
			select {
			case <-uavStopChan:
				log.Println("Closing <- HID goroutine")
				return
			default:
			}

			_, err := cc.Read(buffer)
			if err != nil {
				log.Fatal(err)
			}

			//log.Println(n)
			//printHex(buffer[2:2+buffer[1]], int(buffer[1]))

			packet = append(packet[len(packet):], buffer[2:2+buffer[1]]...)

			for {
				ok, from, to := packetComplete(packet)
				if ok != true {
					break
				}
				if uavTalkObject, err := newUAVTalkObjectFromBinary(packet[from:to]); err == nil {
					//log.Println(uavTalkObject)
					uavChan <- uavTalkObject
				} else {
					log.Println(err)
				}
				copy(packet, packet[from:])
				packet = packet[0 : len(packet)-to]
			}
		}
	}()

	jsonStopChan := make(chan bool)
	// json goroutine
	go func() {
		for {
			select {
			case <-jsonStopChan:
				log.Println("Closing -> HID goroutine")
				return
			case uavTalkObject := <-jsonChan:
				binaryObj, err := uavTalkObject.toBinary()
				if err != nil {
					log.Println(err)
					continue
				}
				binaryObj = append([]byte{0x01, byte(len(binaryObj))}, binaryObj...)
				//printHex(binaryObj, len(binaryObj))

				_, err = cc.Write(binaryObj)
				if err != nil {
					panic(err)
				}
				//log.Println("Bytes sent", n)
			}
		}
	}()

	<-stopChan
	uavStopChan <- true
	jsonStopChan <- true
}

var startStopControlChan = make(chan bool)

func openUAVChan() {
	startStopControlChan <- true
}

func closeUAVChan() {
	startStopControlChan <- false
}

func startUAVTalk(uavChan chan *UAVTalkObject, jsonChan chan *UAVTalkObject) {
	go func() {
		stopChan := make(chan bool)
		started := false
		for startStop := range startStopControlChan {
			if startStop {
				if started == false {
					go startHID(stopChan, uavChan, jsonChan)
					started = true
				}
			} else {
				if started {
					stopChan <- true
					started = false
				}
			}
		}
	}()
}
