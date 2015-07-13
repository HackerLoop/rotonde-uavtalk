# bridge

## TL;DR

In an effort to get [Taulabs flight controller](http://taulabs.org/) usable from any language
this "bridge" had been created. Its main purpose is to get the telemetry coming from
the USB HID connection accessible as a bi-directional stream of JSON over a websocket.

Javascript being a popular language these days, we also created
[Drone.js](https://github.com/openflylab/drone.js) which provides a
basic library to interact with it.

## Installation

### Dependencies

- [Go](http://golang.org/)
- `$GOPATH` properly set
- [*go.hid*](https://github.com/GeertJohan/go.hid) as a dependency requires [HIDAPI
library](https://github.com/signal11/hidapi) to be installed on your
system.
- [mapstructure](https://github.com/mitchellh/mapstructure)

### Compiling

`go build`

### Running

A path to a folder containing UAVobjects definitions must be provided.
You can easily find them by cloning [Taulabs](https://github.com/TauLabs/TauLabs) in the folder `shared/uavobjectdefinition`.

```
$ ./bridge -port 4242 uavobjectdefinition/
2015/06/21 14:43:07 Websocket server started on port 4242
```

Port to listen on can be specified with `-port PORT`.

## Overview

Taulabs' flight controller communication is encapsulated with [UAVTalk](https://wiki.openpilot.org/display/WIKI/UAVTalk) protocol.
Basically, it's a stream of structures carrying drone related data, packaged in a binary format before being sent over the wire. 
These structures are called *UAVObject*s and allows to communicate with
activated modules on the flight controller. 

For example, to control a drone's orientation while flying, an UAVObject
named `ManualControlCommand` can be sent to the flight controller. It
will try to stabilize the drone by adjusting each engines power to match the received settings.
Feel free to browse the exhaustive [list of UAVObjects.](https://gist.github.com/jhchabran/972ad7660398f478d990)

The present project, referenced as the *Bridge* manages the USB
connection to the drone and provides a websocket that streams UAVOBjects
expressed in JSON (they're originally expressed in XML).

So instead of sending something like this:

```
    3C 22 1D 00 E8 B7 75 3F 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 E5
```

The following can be sent : 

```json
{
  "type": "update"
    {
      objectId: 12345,
      instanceId: 0,
      status: 'Connected',
      txDataRate: 0,
      rxDataRate: 0,
      txFailures: 0,
      rxFailures: 0,
      txRetries: 0
    }
}
```

## Using it

In most case, the bridge is used through its a websocket (Rest interface is foreseen), by sending and receiving JSON objects.
There a four types of json objects, "update", "req", "cmd" or "sub",
which are detailed below.

These four json objects all share a common structure :

```json
{
  type: "", // "update", "req", "cmd" or "sub"
  payload: {
    // mixed, based on type
  }
}
```

### Update

The "update" object encapsulate an update of a UAVObject, which can be
found in two different contexts. 

- Received as a notification that a setting had been updated. 
- Sent to update a setting

For example, the attitude module (which is responsible for attitude estimation, which means "what is the current angle of the drone") will periodically send the quaternion representing the current angle of the drone through the AttitudeActual object. But "update" object can also be used to set setting values for the desired module, for example, if you send a AttitudeSettings update object through websocket it will configure the PID algorithm that enables your drone to stay still in the air.

```json
{
  "type": "update",
  "payload": {
    // objectId and instanceId are both required
    "objectId": 1234, // displayed on start of bridge
    "instanceId": 0, // see UAVTalk documentation for info
    "data": {
      // UAVObject data, as described by the definitions
    }
  }
}
```

# Req

Some UAVObjects are sent periodically, like the AttitudeActual that is sent every 100 ms, but others have different update policies, for example, the AttitudeSettings object is sent when changed, which means if you want its value you can either wait for it to change (which should not occure in normal condition), or just request it by sending a "req" object into the pipe, the response will be received as a "update" object.

```json
{
  "type": "req",
  "payload": {
    "objectId": 1234, // displayed on start of bridge, will be received from the def packet
    "instanceId": 0, // see UAVTalk documentation for info
  }
}
```

# Sub

When you connect to the bridge nothing will be received except definitions, you have to subscribe to a given objectId in order to start receiving it.

```json
{
  "type": "sub",
  "payload": {
    "objectId": 1234 // displayed on start of bridge, will be received from the def packet
  }
}
```

# Def

Each uavobject has a set of fields and meta datas, when a uavobject is available (like GPS), the module providing this feature sends its definition to the bridge which then dispatches a definition to the clients. Given that a uavobject reflects an available feature of the drone, definitions give clients a clear overview of the available features.
A client can send definitions to the bridge, exposing the feature that it provides.

```json
{
  "type": "def",
  "payload": {
    // meta datas from uavobject, at first will be tightly linked to definitions found in the xml files
  }
}
```
