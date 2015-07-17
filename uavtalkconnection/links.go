package uavtalkconnection

import (
	"io"
	"net"

	"github.com/GeertJohan/go.hid"
	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/utils"
)

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
		if toWriteLength > maxHIDFrameSize-2 {
			toWriteLength = maxHIDFrameSize - 2
		}
		l.fixedLengthWriteBuffer[0] = 0x02
		l.fixedLengthWriteBuffer[1] = byte(toWriteLength)
		copy(l.fixedLengthWriteBuffer[2:], b[currentOffset:currentOffset+toWriteLength])
		log.Info("sending")
		utils.PrintHex(l.fixedLengthWriteBuffer, len(l.fixedLengthWriteBuffer))
		n, err := l.cc.Write(l.fixedLengthWriteBuffer)
		if err != nil {
			return currentOffset, err
		}
		currentOffset += n
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
	b = b[2 : 2+b[1]]
	return len(b), nil
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
