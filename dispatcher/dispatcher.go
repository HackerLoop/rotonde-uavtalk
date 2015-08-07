package dispatcher

import (
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/openskybot/skybot-router/common"
)

// ChanQueueLength buffered channel length
const ChanQueueLength = 10

// Object native representation of a UAVPacket, just a map
type Object map[string]interface{}

// Update is the UAVTalk protocol packet, encapsulates a common in the field Data
type Update struct {
	ObjectID   uint32 `json:"objectId"`
	InstanceID uint16 `json:"instanceId"`
	Data       Object `json:"data"`
}

// Request is the packet that requests a common data, is forwarded to the owner of a definition (the one that sent the definition)
type Request struct {
	ObjectID   uint32 `json:"objectId"`
	InstanceID uint16 `json:"instanceId"`
}

// Subscription adds an objectID to the subscriptions of the sending connection
type Subscription struct {
	ObjectID uint32 `json:"objectId"`
}

// Unsubscription removes an objectID from the subscriptions of the sending connection
type Unsubscription struct {
	ObjectID uint32 `json:"objectId"`
}

// Connection : basic interface representing a connection to the dispatcher
type Connection struct {
	definitions   common.Definitions
	subscriptions []uint32
	InChan        chan interface{}
	OutChan       chan interface{}
}

// NewConnection creates a new dispatcher connection
func NewConnection() *Connection {
	connection := new(Connection)
	connection.InChan = make(chan interface{}, ChanQueueLength)
	connection.OutChan = make(chan interface{}, ChanQueueLength)

	return connection
}

// Close closes the connection, possible issues...
func (connection *Connection) Close() {
	close(connection.OutChan)
}

func (connection *Connection) addSubscription(objectID uint32) {
	connection.subscriptions = append(connection.subscriptions, objectID)
}

func (connection *Connection) removeSubscription(objectID uint32) {
	for i, subscription := range connection.subscriptions {
		if subscription == objectID {
			if i < len(connection.subscriptions)-1 {
				copy(connection.subscriptions[i:], connection.subscriptions[i+1:])
			}
			connection.subscriptions = connection.subscriptions[0 : len(connection.subscriptions)-1]
			return
		}
	}
}

// Dispatcher main dispatcher class
type Dispatcher struct {
	connections    []*Connection
	cases          []reflect.SelectCase // cases for the select case of the main loop, the first element il for the connectionChan, the others are for the outChans of the connections
	connectionChan chan *Connection     // connectionChan receives the new connections to add
}

// NewDispatcher creates a dispatcher
func NewDispatcher() *Dispatcher {
	dispatcher := new(Dispatcher)
	dispatcher.connections = make([]*Connection, 0, 100)
	dispatcher.cases = make([]reflect.SelectCase, 0, 100)
	dispatcher.connectionChan = make(chan *Connection, 10) // TODO try unbuffered chan

	// first case is for the connectionChan
	dispatcher.cases = append(dispatcher.cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(dispatcher.connectionChan)})

	return dispatcher
}

// AddConnection adds a connection to the dispatcher
func (dispatcher *Dispatcher) AddConnection(connection *Connection) {
	dispatcher.connectionChan <- connection
}

func (dispatcher *Dispatcher) addConnection(connection *Connection) {
	for _, c := range dispatcher.connections {
		for _, d := range c.definitions {
			connection.InChan <- *d
		}
	}

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
	fromConnection := dispatcher.connections[from]
	fromOwner := false
	if _, err := fromConnection.definitions.GetDefinitionForObjectID(update.ObjectID); err == nil {
		fromOwner = true
	}
mainLoop:
	for i, connection := range dispatcher.connections {
		if i == from {
			continue
		}

		if fromOwner == true {
			for _, objectID := range connection.subscriptions {
				if objectID == update.ObjectID {
					connection.InChan <- *update
					continue mainLoop
				}
			}
		} else {
			if _, err := connection.definitions.GetDefinitionForObjectID(update.ObjectID); err == nil {
				connection.InChan <- *update
			}
		}
	}
}

func (dispatcher *Dispatcher) dispatchDefinition(from int, definition *common.Definition) {
	for i, connection := range dispatcher.connections {
		if i == from {
			continue
		}
		connection.InChan <- *definition
	}
}

func (dispatcher *Dispatcher) dispatchRequest(request *Request) {
	for _, connection := range dispatcher.connections {
		if _, err := connection.definitions.GetDefinitionForObjectID(request.ObjectID); err == nil {
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
			dispatcher.dispatchUpdate(chosen-1, &data)
		case Subscription:
			log.Info("Executing subscribe")
			connection := dispatcher.connections[chosen-1]
			connection.addSubscription(data.ObjectID)
		case Unsubscription:
			log.Info("Executing unsubscribe")
			connection := dispatcher.connections[chosen-1]
			connection.removeSubscription(data.ObjectID)
		case common.Definition:
			log.Info("Dispatching Definition message")
			connection := dispatcher.connections[chosen-1]
			connection.definitions = append(connection.definitions, &data)
			dispatcher.dispatchDefinition(chosen-1, &data)
		case Request:
			log.Info("Dispatching Request message")
			dispatcher.dispatchRequest(&data)
		case *Connection:
			log.Info("Add connection")
			dispatcher.addConnection(data) // data is already a pointer
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
