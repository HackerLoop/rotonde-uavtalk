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

### Compiling

`go build`

### Running 

A path to a folder containing UAVobjects definitions must be provided.
You can easily find them by cloning [Taulabs](https://github.com/TauLabs/TauLabs) in the folder `shared/uavobjectdefinition`.

```
$ ./bridge uavobjectdefinition/
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
      Status: 'Connected',
      TxDataRate: 0,
      RxDataRate: 0,
      TxFailures: 0,
      RxFailures: 0,
      TxRetries: 0
    }

