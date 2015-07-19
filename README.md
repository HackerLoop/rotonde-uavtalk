# TL;DR

Control drone as a device through JSON websocket API.

# Intro

This document describes the openskybot bridge and its underlying mecanism, if you just want to control the drone,
head to [Drone.js](https://github.com/openflylab/drone.js) (coming soon) which provides a much more user-friendly interface.

The main idea of this project is give easy access to features like "take me to this coordinates", just like
you have access to a "print this document" feature on most computers.
So we thought that it would be nice if we could plug a computer to a drone frame, and just have it flying in a snap,
so we could just focus on what can be done with a computer once it's flying.

Having a machine flying, whether it is a quad rotor or a plane, requires strong mathematical knowledge in a piece of software called a `Flight controller`. This project relies on [Taulabs](http://taulabs.org/) to fulfill this task, and thus requires an actual piece of hardware to run the flight controller on, just connect it to your embedded computer by USB, like any printer or mouse.

# Setup (assuming you are on a unix machine, tested on OSX and linux)

## Compilation

First you will need to install the [Golang](http://golang.org/) programming language (I'm working on go1.4.2, not tested others, let me know),

Once installed, copy-paste in terminal, replace [ choose_workspace_directory ] by your desired path of installation:
```bash
export GOPATH=[ choose_workspace_directory ]
mkdir $GOPATH
go get github.com/openflylab/bridge && go get github.com/tools/godep
cd $GOPATH/src/github.com/openflylab/bridge
godep restore
go build
```

You now have an executable called `bridge` in src/github.com/openflylab/bridge from the installation path,
just head to the next section to know how to launch it.

If something went wrong, please post the resut to these commands in a new issue.

## Running

A path to a folder containing UAVObjects definitions must be provided.
You can easily find them by cloning [Taulabs](https://github.com/TauLabs/TauLabs) in the folder `shared/UAVObjectdefinition`.

Execute following commands in the same directory as the `bridge` executable previously created,
replace [ path_to_the_cloned_TauLabs_folder ] with the place you wish to clone Taulabs sources.
```bash
export TAULABS=[ path_to_the_cloned_TauLabs_folder ]
git clone https://github.com/TauLabs/TauLabs.git $TAULABS
./bridge $TAULABS/shared/UAVObjectdefinition
```

Default port is 4224, port to listen on can be specified with `-port PORT`.

# Overview

The Taulabs flight controller software uses a very handy modular architecture, each modules are abstracted from
each others, and communicate by sending and receiving UAVObjects through a common bus.
Each module exposes hist functionalities and settings through one or more *UAVObject*s. Modules can modify other
modules' UAVObjects to communicate with each other.

TODO: insert graphic ?

For example, connecting to the bus would give you the ability to update the [AltitudeHoldDesired](https://raw.githubusercontent.com/TauLabs/TauLabs/next/shared/uavobjectdefinition/altitudeholddesired.xml) UAVObject, that contains an `Altitude` field, and give you control over the desired drone altitude.

Feel free to browse the exhaustive [list of UAVObjects.](https://gist.github.com/jhchabran/972ad7660398f478d990)

The present project, referenced as the *Bridge* manages the USB
connection to the drone and provides a websocket that streams *UAVObject*s
expressed in JSON (they're originally expressed in binary).

So instead of sending something like this:

```
    3C 22 1D 00 E8 B7 75 3F 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 E5
```

The following can be sent :

```json
{
  "type": "update"
    {
      "objectId": 12345,
      "instanceId": 0,
      "status": "Connected",
      "txDataRate": 0,
      "rxDataRate": 0,
      "txFailures": 0,
      "rxFailures": 0,
      "txRetries": 0
    }
}
```

## JSON protocol

In most case, the bridge is used through its websocket (Rest interface is foreseen), by sending and receiving JSON objects.
There a five types of json objects, "update", "req", "cmd", "sub" or "unsub",
which are detailed below.

These four json objects all share a common structure :

```json
{
  type: "", // "update", "req", "cmd", "sub" or "unsub"
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

For example, the attitude module (which is responsible for attitude estimation, which means "what is the current angle of the drone") will periodically send the quaternion representing the current angle of the drone through the AttitudeActual object.

But "update" objects can also be used to set setting values for the desired module, for example, if you send a [AttitudeSettings](https://raw.githubusercontent.com/TauLabs/TauLabs/next/shared/uavobjectdefinition/attitudesettings.xml) update object through websocket it will configure the PID algorithm that enables your drone to stay still in the air.

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

### Req

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

### Sub / Unsub

When you connect to the bridge nothing will be received except definitions, you have to subscribe to a given objectId in order to start receiving its updates.

```json
{
  "type": "sub",
  "payload": {
    "objectId": 1234 // displayed on start of bridge, will be received from the def packet
  }
}
```

and you can unsubscribe from this objectId with:

```json
{
  "type": "unsub",
  "payload": {
    "objectId": 1234 // displayed on start of bridge, will be received from the def packet
  }
}
```

### Def

Each UAVObject has a set of fields and meta datas, when a UAVObject is available (like GPS), the module providing this feature sends its definition to the bridge which then dispatches a definition to the clients.
Given that a UAVObject reflects an available feature of the drone, definitions give clients a clear overview of the available features.
A client can send definitions to the bridge, exposing the feature that it provides.

When you connect to the bridge, it will start be sending you all the currently available definitions, new definitions can still become available at any time.

```json
{
  "type": "def",
  "payload": {
    // meta datas from UAVObject, at first will be tightly linked to definitions found in the xml files
  }
}
```

# Tutorial #1

Let's say we want to make a quick monitoring app that shows a 3D representation of our drone in air.
We'd need to have the current 3d angle of our drone. The good news is that we actually have a UAVObject just for that !
It's called [AttitudeActual](https://raw.githubusercontent.com/TauLabs/TauLabs/next/shared/uavobjectdefinition/attitudeactual.xml), and the 3D angle is given in two forms, as Euler angle with the Yaw, Pitch and Roll fields, and as a quaternion with the q1, q2, q3 and q4 fields.

so let's start by connecting to the websocket, which will start by sending us all available definitions one by one.
Once we reach the AttitudeActual definition:

```json
{
   "type":"def",
   "payload":{
      "name":"AttitudeActual",
      "description":"The updated Attitude estimation from @ref AHRSCommsModule.",
      "singleInstance":true,
      "settings":false,
      "category":"",
      "id":869979622,
      "access":{
         "gcs":"readwrite",
         "flight":"readwrite"
      },
      "telemetryGcs":{
         "acked":false,
         "updateMode":"manual",
         "period":"0"
      },
      "telemetryFlight":{
         "acked":false,
         "updateMode":"periodic",
         "period":"100"
      },
      "logging":{
         "updateMode":"manual",
         "period":"0"
      },
      "fields":[
         {
            "name":"q1",
            "units":"",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         },
         {
            "name":"q2",
            "units":"",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         },
         {
            "name":"q3",
            "units":"",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         },
         {
            "name":"q4",
            "units":"",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         },
         {
            "name":"Roll",
            "units":"degrees",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         },
         {
            "name":"Pitch",
            "units":"degrees",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         },
         {
            "name":"Yaw",
            "units":"degrees",
            "FieldTypeInfo":{
               "Index":6,
               "Name":"float",
               "Size":4
            },
            "elements":1,
            "elementsName":null,
            "options":null,
            "defaultValue":"",
            "cloneOf":""
         }
      ]
   }
}
```

Yes, there is a lot on infos there, a big part of it will be stripped in futur versions.
Anyway the one field that interests us is the `id` field, 869979622. This is the id we want to subscribe to.

Let's go on and subscribe; create a json paket with the following format:

```json
{
    "type": "sub",
    "payload": {
        "objectId": 869979622
    }
}
```

send it to the dispatcher and it will start sending you the AttitudeActual object periodically. yay.
It looks like this:

```json
{
   "type":"update",
   "payload":{
      "objectId":869979622,
      "instanceId":0,
      "data":{
         "Pitch":-34.833576,
         "Roll":92.87233,
         "Yaw":67.75378,
         "q1":0.42503926,
         "q2":0.688997,
         "q3":0.21412396,
         "q4":0.5466039
      }
   }
}
```

###Exercise

Try to unsubscribe from this packet.

# Contribution

We need people to develop new modules that provide crazy UAVObjects !
For example, have look at the [VelocityActual](https://raw.githubusercontent.com/TauLabs/TauLabs/next/shared/uavobjectdefinition/velocityactual.xml) UAVObject which provides the velocity as seen through the various sensors of the hardware, which are not necessarily perfectly accurate.
What if you could have a better estimation, like when you are close enough to the ground to have optical based velocity estimation, which is much more accurate ? Then you could just update this UAVObject and provide much more accurate velocity estimation to the on-board algorithm which would result in a much more accurate drone !

So that's the whole point of the modular architecture that Taulabs offers, some modules work perfectly on the flight controller, but others might require much more processing power, or an internet connection, these one need be on an actual computer, some other modules might even require to be in the cloud.
The modular architecture provides a clean abstraction that makes all this possible, just connect to the bridge, manipulate UAVObjects, and your are part of the whole system.
