package usbconnection

import (
	log "github.com/Sirupsen/logrus"
	"github.com/openflylab/bridge/dispatcher"
)

type sessionWrapper struct {
	connection *dispatcher.Connection
	inChan     chan interface{}
	outChan    chan interface{}
}

func createGCSTelemetryStatsUpdate(status string) dispatcher.Update {
	definition, err := definitions.GetDefinitionForName("GCSTelemetryStats")
	if err != nil {
		log.Fatal(err)
	}
	packet := dispatcher.Update{
		ObjectID:   definition.ObjectID,
		InstanceID: 0,
		Data: map[string]interface{}{
			"Status":     status,
			"TxDataRate": float64(0),
			"RxDataRate": float64(0),
			"TxFailures": float64(0),
			"RxFailures": float64(0),
			"TxRetries":  float64(0),
		},
	}
	return packet
}

/*func createSessionManaging(status string) dispatcher.Update {
	definition, err := definitions.GetDefinitionForName("GCSTelemetryStats")
	if err != nil {
		log.Fatal(err)
	}
	packet := dispatcher.Update{
		ObjectID:   definition.ObjectID,
		InstanceID: 0,
		Data: map[string]interface{}{
			"Status":     status,
			"TxDataRate": float64(0),
			"RxDataRate": float64(0),
			"TxFailures": float64(0),
			"RxFailures": float64(0),
			"TxRetries":  float64(0),
		},
	}
	return packet
}*/

func newSessionWrapper() *sessionWrapper {
	sw := sessionWrapper{}
	sw.connection = dispatcher.NewConnection()
	sw.inChan = sw.connection.InChan
	sw.outChan = make(chan interface{}, dispatcher.ChanQueueLength)

	handshakeReq := createGCSTelemetryStatsUpdate("HandshakeReq")
	sw.inChan <- handshakeReq

	flightTelemetryStats, err := definitions.GetDefinitionForName("FlightTelemetryStats")
	if err != nil {
		log.Fatal(err)
	}

	sessionManaging, err := definitions.GetDefinitionForName("SessionManaging")
	if err != nil {
		log.Fatal(err)
	}

	connected := false
	// outChan -> OutChan
	go func() {
		for {
			message := <-sw.outChan
			switch m := message.(type) {
			case dispatcher.Update:
				if m.ObjectID == flightTelemetryStats.ObjectID {
					log.Info("Received: ", m.Data["Status"])
					switch {
					case m.Data["Status"] == "HandshakeAck" || m.Data["Status"] == "Connected":
						handshakeConnected := createGCSTelemetryStatsUpdate("Connected")
						sw.inChan <- handshakeConnected
						if connected == false {
							// TODO send SessionManaging ?
						}
						connected = true
					}
				} else if m.ObjectID == sessionManaging.ObjectID {
					log.Info(m)
				}
			}
			sw.connection.OutChan <- message
		}
	}()

	return &sw
}
