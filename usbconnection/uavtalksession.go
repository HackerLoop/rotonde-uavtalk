package usbconnection

import "github.com/openflylab/bridge/dispatcher"

type sessionWrapper struct {
	connection *dispatcher.Connection
	inChan     chan interface{}
	outChan    chan interface{}
}

func newSessionWrapper() *sessionWrapper {
	sw := sessionWrapper{}
	sw.connection = dispatcher.NewConnection()
	sw.inChan = sw.connection.InChan
	sw.outChan = make(chan interface{}, dispatcher.ChanQueueLength)

	// outChan -> OutChan
	go func() {
		for {
			message := <-sw.outChan
			// TODO manage session and connection related UAVO
			sw.connection.OutChan <- message
		}
	}()

	return &sw
}
