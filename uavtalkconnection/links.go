package uavtalkconnection

import (
	"errors"
	"io"
	"net"

	"github.com/GeertJohan/go.hid"
)

/**
 * This file contains a simple abstraction layer for the telemetry link (eg. how are we connecting to the controller ?)
 * It currently supports USB HID and TCP links.
 */

type linker interface {
	io.Reader
	io.Writer
	io.Closer
}

type usbLink struct {
	cc                     *hid.Device
	fixedLengthWriteBuffer []byte
}

var _ linker = (usbLink{})

var deviceIDs = []struct {
	vendorID  uint16
	productID uint16
}{
	{vendorID: 0x20a0, productID: 0x415b}, // cc3d, flyingf4, sparky2
	{vendorID: 0x20a0, productID: 0x4195}, // flyingf3
	{vendorID: 0x20a0, productID: 0x41d0}, // sparky
	{vendorID: 0x20a0, productID: 0x415c}, // taulink, does it mean anything to put it here ? pipxtreme
	{vendorID: 0x20a0, productID: 0x4235}, // colibri
	{vendorID: 0x0fda, productID: 0x0100}, // quanton
}

func newUSBLink() (linker, error) {
	devices, err := hid.Enumerate(0x00, 0x00)
	if err != nil {
		return nil, err
	}

	var deviceIDIndex = -1
Loop:
	for _, device := range devices {
		for index, deviceID := range deviceIDs {
			if device.VendorId == deviceID.vendorID && device.ProductId == deviceID.productID {
				deviceIDIndex = index
				break Loop
			}
		}
	}
	if deviceIDIndex == -1 {
		return nil, errors.New("No suitable device found")
	}

	device := deviceIDs[deviceIDIndex]
	cc, err := hid.Open(device.vendorID, device.productID, "")
	if err != nil {
		return nil, err
	}

	return usbLink{cc, make([]byte, maxHIDFrameSize)}, nil
}

func (l usbLink) Write(b []byte) (int, error) {
	currentOffset := 0
	for currentOffset < len(b) {
		toWriteLength := len(b) - currentOffset
		// packet on the HID link can't be > maxHIDFrameSize, split it if it's the case.
		if toWriteLength > maxHIDFrameSize-2 {
			toWriteLength = maxHIDFrameSize - 2
		}

		// USB HID link requires a reportID and packet length as first bytes
		l.fixedLengthWriteBuffer[0] = 0x02
		l.fixedLengthWriteBuffer[1] = byte(toWriteLength)
		copy(l.fixedLengthWriteBuffer[2:], b[currentOffset:currentOffset+toWriteLength])

		n, err := l.cc.Write(l.fixedLengthWriteBuffer)
		if err != nil {
			return currentOffset, err
		}
		if n > 2 {
			currentOffset += n - 2
		}
	}
	return currentOffset, nil
}

func (l usbLink) Read(b []byte) (int, error) {
	n, err := l.cc.ReadTimeout(b, 50)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	s := int(b[1])
	copy(b, b[2:]) // this sucks...

	return s, nil
}

func (l usbLink) Close() error {
	l.cc.Close()
	return nil
}

type tcpLink net.Conn

var _ linker = (tcpLink)(nil)

func newTCPLink() (linker, error) {
	return net.Dial("tcp", "localhost:9000")
}
