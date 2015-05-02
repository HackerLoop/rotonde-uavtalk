package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/GeertJohan/go.hid"
)

const VER_MASK = 0x20

type UAVTalkObject struct {
	cmd        uint8
	length     uint16
	objectId   uint32
	instanceId uint16
	data       []byte
	cks        uint8
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

func packetComplete(packet []byte) (bool, int) {
	if packet[0] != 0x3c {
		return false, 0
	}

	length := byteArrayToInt16(packet[2:4])

	if int(length)+1 > len(packet) {
		return false, 0
	}

	cks := packet[length]

	// check cks
	// fmt.Printf("%d %d\n", uint8(cks), computeCrc8(0, packet[0:length]))

	if cks != computeCrc8(0, packet[0:length]) {
		return false, 0
	}

	return true, int(length) + 1
}

func newUAVTalkObject(packet []byte) (*UAVTalkObject, error) {
	if packet[0] != 0x3c {
		return nil, errors.New("Wrong Sync val xP")
	}

	uavTalkObject := &UAVTalkObject{}

	uavTalkObject.cmd = packet[1] ^ VER_MASK
	uavTalkObject.length = byteArrayToInt16(packet[2:4])
	uavTalkObject.objectId = byteArrayToInt32(packet[4:8])
	uavTalkObject.instanceId = byteArrayToInt16(packet[8:10])

	uavTalkObject.data = make([]byte, uavTalkObject.length-10)
	copy(uavTalkObject.data, packet[10:len(packet)-1])

	uavTalkObject.cks = packet[len(packet)-1]

	//fmt.Println(uavTalkObject)

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

func startHID(c chan *UAVTalkObject) {
	cc, err := hid.Open(0x20a0, 0x415b, "")
	if err != nil {
		log.Fatal(err)
	}
	defer cc.Close()

	buffer := make([]byte, 64)
	packet := make([]byte, 0, 4096)
	for {
		n, err := cc.Read(buffer)
		if err != nil {
			panic(err)
		}

		packet = append(packet[len(packet):], buffer[2:n]...)

		for {
			ok, n := packetComplete(packet)
			if ok != true {
				break
			}
			//printHex(packet[:n], n)
			if uavTalkObject, err := newUAVTalkObject(packet[:n]); err == nil {
				c <- uavTalkObject
			} else {
				fmt.Println(err)
			}
			copy(packet, packet[n:])
			packet = packet[0 : len(packet)-n]
		}
	}
}

func startUAVTalk() chan *UAVTalkObject {
	c := make(chan *UAVTalkObject)

	go startHID(c)

	return c
}
