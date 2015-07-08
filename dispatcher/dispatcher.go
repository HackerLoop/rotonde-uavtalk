package dispatcher

import (
	"reflect"

	"github.com/openflylab/bridge/uavobject"

	log "github.com/Sirupsen/logrus"
)

const chanQueueLength = 100

// Object native representation of a UAVPacket, just a map
type Object map[string]interface{}

// Update is the UAVTalk protocol packet, encapsulates a UAVObject in the field Data
type Update struct {
	ObjectID   uint32 `json:"objectId"`
	InstanceID uint16 `json:"instanceId"`
	Data       Object `json:"data"`
}

// Request is the packet that requests a uavobject data, is forwarded to the owner of a UAVObject (the one the sent the definition)
type Request struct {
	ObjectID   uint32 `json:"objectId"`
	InstanceID uint16 `json:"instanceId"`
}

// Command : Dispatcher command
type Command interface {
	Execute(connection *Connection, dispatcher *Dispatcher) bool
}

// Connection : basic interface representing a connection to the dispatcher
type Connection struct {
	definitions uavobject.Definitions
	inFilters   []Filter
	outFilters  []Filter
	InChan      chan interface{}
	OutChan     chan interface{}
}

// NewConnection creates a new dispatcher connection
func NewConnection() *Connection {
	connection := new(Connection)
	connection.InChan = make(chan interface{}, chanQueueLength)
	connection.OutChan = make(chan interface{}, chanQueueLength)

	return connection
}

// Dispatcher main dispatcher class
type Dispatcher struct {
	connections []*Connection
	cases       []reflect.SelectCase
}

// NewDispatcher creates a dispatcher
func NewDispatcher() *Dispatcher {
	dispatcher := new(Dispatcher)
	dispatcher.connections = make([]*Connection, 0, 100)
	dispatcher.cases = make([]reflect.SelectCase, 0, 100)

	// dispatcher.cases = append(dispatcher.cases, reflect.SelectCase{Dir: reflect.SelectDefault})

	return dispatcher
}

// AddConnection adds a connection to the dispatcher
func (dispatcher *Dispatcher) AddConnection(connection *Connection) {
	dispatcher.connections = append(dispatcher.connections, connection)
	dispatcher.cases = append(dispatcher.cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(connection.OutChan)})
}

func (dispatcher *Dispatcher) dispatchUpdate(from int, update *Update) {
	talkingConnection := dispatcher.connections[from]
	for _, filter := range talkingConnection.outFilters {
		if filter.PassThrough(*update) == false {
			return
		}
	}

	for i, connection := range dispatcher.connections {
		if i == from {
			continue
		}
		for _, filter := range connection.inFilters {
			if filter.PassThrough(*update) == false {
				return
			}
		}
		connection.InChan <- *update
	}
}

func (dispatcher *Dispatcher) dispatchRequest(request *Request) {

}

func (dispatcher *Dispatcher) processChannels() {
	chosen, value, ok := reflect.Select(dispatcher.cases)
	if !ok {
		log.Warning("One of the channels is broken.", chosen)
		// TODO remove Connection + Select Case
	} else {
		connection := dispatcher.connections[chosen]
		switch data := value.Interface().(type) {
		case Update:
			dispatcher.dispatchUpdate(chosen, &data)
		case Command:
			data.Execute(connection, dispatcher)
		case uavobject.Definition:
			connection.definitions = append(connection.definitions, &data)
			for i, connection := range dispatcher.connections {
				if i == chosen {
					continue
				}
				connection.InChan <- data
			}
		default:
			log.Warning("Oops got some unknown object in the dispatcher, ignoring.")
		}
	}
}

// Start starts the dispatcher
func (dispatcher *Dispatcher) Start() {
	go (func() {
		for {
			dispatcher.processChannels()
		}
	})()
}
