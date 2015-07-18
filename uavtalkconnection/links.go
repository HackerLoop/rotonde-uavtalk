package uavtalkconnection

import (
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

func newUSBLink() (linker, error) {
	cc, err := hid.Open(0x20a0, 0x41d0, "")
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
