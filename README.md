# bridge

## TL;DR

In an effort to get the taulabs flight controller usable from (but not exclusively) javascript,
we created this bridge which main purpose is to get the telemetry coming from
the USB HID connection accessible as a bi-directional stream of JSON over a websocket. *breathe*

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

## How it works

Taulabs' flight controller communication is packaged in the [UAVTalk](https://wiki.openpilot.org/display/WIKI/UAVTalk) protocol,
which is basically a stream of objects sent as binary data. These objects are called UAVObjects,
and give the ability to communicate with the modules currently activated on the flight controller.
For example, updating the ManualControlCommand UAVObject's attributes gives control over the drone orientation in air,
the flight controller managing the engines' power to stabilize the drone.

- [list of
  UAVObjects](https://gist.github.com/jhchabran/972ad7660398f478d990)

What the bridge does is manage the USB connection, and give you a clean JSON websocket
through UAVObjects transit as JSON, instead of binary.

So instead of sending something like this:

    3C 22 1D 00 E8 B7 75 3F 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 E5

You just send something like this:

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

## How to use

(Please read "How it works" first)
In most case, the bridge is used through its websocket interfaces (Rest interface is foreseen), by sending and receiving JSON objects.
There a four types of json objects, "update", "req", "cmd" or "sub".

A basic packet is as follows:

{
  type: "", // "update", "req", "cmd" or "sub"
  payload: {
    // mixed, based on type
  }
}

# Update

The "update" type represents an update of a UAVObject, they can occure in two circumstances. For example, the attitude module (which is responsible for attitude estimation, which means "what is the current angle of the drone") will periodically send the quaternion representing the current angle of the drone through the AttitudeActual object. But "update" object can also be used to set setting values for the desired module, for example, if you send a AttitudeSettings update object through websocket it will configure the PID algorithm that enables your drone to stay still in the air.

{
  "type": "update",
  "payload": {
    // objectId and instanceId are both required
    "objectId": 1234, // displayed on start of bridge
    "instanceId": 0, // see UAVTalk documentation for info
    "data": {
      // UAVObject data, see xml definitions for reference
    }
  }
}

# Req

Some UAVObjects are sent periodically, like the AttitudeActual that is sent every 100 ms, but others have different update policies, for example, the AttitudeSettings object is sent when changed, which means if you want its value you can either wait for it to change (which should not occure in normal condition), or just request it by sending a "req" object into the pipe, the response will be received as a "update" object.

{
  "type": "req",
  "payload": {
    "objectId": 1234, // displayed on start of bridge, will be received from the def packet
    "instanceId": 0, // see UAVTalk documentation for info
  }
}

# Sub

When you connect to the bridge nothing will be received except definitions, you have to subscribe to a given objectId in order to start receiving it.

{
  "type": "sub",
  "payload": {
    "objectId": 1234 // displayed on start of bridge, will be received from the def packet
  }
}

# Def

Each uavobject has a set of fields and meta datas, when a uavobject is available (like GPS), the module providing this feature sends its definition to the bridge which then dispatches a definition to the clients. Given that a uavobject reflects an available feature of the drone, definitions give clients a clear overview of the available features.
A client can send definitions to the bridge, exposing the feature that it provides.

{
  "type": "def",
  "payload": {
    // meta datas from uavobject, at first will be tightly linked to definitions found in the xml files
  }
}
