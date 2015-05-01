package main

import (
	"errors"
	"fmt"

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

func (uavTalkObject *UAVTalkObject) isComplete() bool {
	return int(uavTalkObject.length) <= (len(uavTalkObject.data) + 10)
}

func (uavTalkObject *UAVTalkObject) appendBuffer(buffer []byte) ([]byte, error) {
	var requiredSize = int(uavTalkObject.length) - 10
	var left = requiredSize - len(uavTalkObject.data)

	//fmt.Printf("length: %d\ndataSize: %d\nbufferSize: %d\nrequiredSize: %d\nleft: %d\n\n", uavTalkObject.length, len(uavTalkObject.data), len(buffer), requiredSize, left)
	if left > len(buffer) {
		left = len(buffer)
	} else {
		//fmt.Printf("%d - %d = %d\n", len(buffer), left, len(buffer)-left)
		uavTalkObject.cks = buffer[left]
	}

	uavTalkObject.data = append(uavTalkObject.data, buffer[:left]...)
	return buffer[left:], nil
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

func newUAVTalkObject(buffer []byte) (*UAVTalkObject, []byte, error) {
	offset := 2

	if buffer[offset] != 0x3c {
		return nil, nil, errors.New("Wrong Sync val xP")
	}

	uavTalkObject := &UAVTalkObject{}

	uavTalkObject.cmd = buffer[offset+1] ^ VER_MASK
	uavTalkObject.length = byteArrayToInt16(buffer[offset+2 : offset+4])
	uavTalkObject.objectId = byteArrayToInt32(buffer[offset+4 : offset+8])
	uavTalkObject.instanceId = byteArrayToInt16(buffer[offset+8 : offset+10])

	left, err := uavTalkObject.appendBuffer(buffer)
	if err != nil {
		return nil, nil, err
	}

	//fmt.Println(uavTalkObject)

	return uavTalkObject, left, nil
}

func printHex(buffer []byte, n int) {
	for i := 0; i < n; i++ {
		if i > 0 {
			fmt.Print(":")
		}
		fmt.Printf("%.02x", buffer[i])
	}
	fmt.Println()
}

func startHID(c chan *UAVTalkObject) {
	cc, err := hid.Open(0x20a0, 0x415b, "")
	if err != nil {
		panic(err)
	}
	defer cc.Close()

	var uavTalkObject *UAVTalkObject
	var left = make([]byte, 64)
	for {
		buffer := make([]byte, 64)
		n, err := cc.Read(buffer)
		if err != nil {
			panic(err)
		}

		//printHex(buffer[0:n], n)

		if uavTalkObject == nil {
			uavTalkObject, left, err = newUAVTalkObject(buffer[0:n])
			if err != nil {
				fmt.Println(err)
				continue
			}
		} else {
			left, err := uavTalkObject.appendBuffer(buffer[0:n])
			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		if uavTalkObject != nil && uavTalkObject.isComplete() {
			c <- uavTalkObject
			uavTalkObject = nil
		}
	}
}

func startUAVTalk() chan *UAVTalkObject {
	c := make(chan *UAVTalkObject)

	go startHID(c)

	return c
}
