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

// Close closes the connection, possible threading issues...
func (connection *Connection) Close() {
	close(connection.OutChan)
}

// Dispatcher main dispatcher class
type Dispatcher struct {
	connections    []*Connection
	cases          []reflect.SelectCase
	connectionChan chan *Connection
}

// NewDispatcher creates a dispatcher
func NewDispatcher() *Dispatcher {
	dispatcher := new(Dispatcher)
	dispatcher.connections = make([]*Connection, 0, 100)
	dispatcher.cases = make([]reflect.SelectCase, 0, 100)
	dispatcher.connectionChan = make(chan *Connection, 10)

	dispatcher.cases = append(dispatcher.cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(dispatcher.connectionChan)})

	return dispatcher
}

// AddConnection adds a connection to the dispatcher
func (dispatcher *Dispatcher) AddConnection(connection *Connection) {
	dispatcher.connectionChan <- connection
}

func (dispatcher *Dispatcher) addConnection(connection *Connection) {
	// the connections slice might have empty spaces, if so, we fill them with new arrivals
	dispatcher.connections = append(dispatcher.connections, connection)
	dispatcher.cases = append(dispatcher.cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(connection.OutChan)})
}

func (dispatcher *Dispatcher) removeConnectionAt(index int) {
	// if it is not the last element, move all next elements
	if index < len(dispatcher.connections) {
		copy(dispatcher.connections[index:], dispatcher.connections[index+1:])
		copy(dispatcher.cases[index+1:], dispatcher.cases[index+2:])
	}
	dispatcher.connections[len(dispatcher.connections)-1] = nil
	dispatcher.connections = dispatcher.connections[:len(dispatcher.connections)-1]

	dispatcher.cases = dispatcher.cases[:len(dispatcher.cases)-1]
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

func (dispatcher *Dispatcher) dispatchDefinition(from int, definition *uavobject.Definition) {
	for i, connection := range dispatcher.connections {
		if i == from {
			continue
		}
		connection.InChan <- *definition
	}
}

func (dispatcher *Dispatcher) dispatchRequest(request *Request) {
	for _, connection := range dispatcher.connections {
		if _, err := connection.definitions.GetDefinitionForObjectID(request.ObjectID); err != nil {
			connection.InChan <- *request
			return
		}
	}
}

func (dispatcher *Dispatcher) processChannels() {
	chosen, value, ok := reflect.Select(dispatcher.cases)
	if !ok {
		log.Warning("One of the channels is broken.", chosen)
		dispatcher.removeConnectionAt(chosen - 1)
	} else {
		switch data := value.Interface().(type) {
		case Update:
			log.Info("Dispatching Update message")
			dispatcher.dispatchUpdate(chosen-1, &data)
		case Command:
			log.Info("Executing command")
			connection := dispatcher.connections[chosen-1]
			data.Execute(connection, dispatcher)
		case uavobject.Definition:
			log.Info("Dispatching Definition message")
			connection := dispatcher.connections[chosen-1]
			connection.definitions = append(connection.definitions, &data)
			dispatcher.dispatchDefinition(chosen-1, &data)
		case Request:
			log.Info("Dispatching Request message")
			dispatcher.dispatchRequest(&data)
		case *Connection:
			log.Info("Add connection")
			dispatcher.addConnection(data)
		default:
			log.Warning("Oops got some unknown object in the dispatcher, ignoring.")
		}
	}
}

// Start starts the dispatcher
func (dispatcher *Dispatcher) Start() {
	for {
		dispatcher.processChannels()
	}
}
