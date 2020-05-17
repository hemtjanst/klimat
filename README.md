# 🌡️ Klimat ❄️

Klimat, or climate in Swedish, can be used to turn devices that help you
manage and monitor the indoor climate into Hemtjänst sensors and make them
controllable.

Right now the only supported thing is reading the values from the Philips
AC 3829/10. It is expected to work with any Philips device in this
category, that uses the CoAP with encrypted payloads protocol/firmware.

It is worth noting that these devices automatically send all their sensor
data to Philips using AWS IoT, regardless of whether you are logged in to
a Philips account. This is how the AirMatters app displays historical values.
If you find this creepy, you can isolate the device in its own VLAN and deny
it access to the outside world. Everything will keep working, including the
AirMatters app, except for it no longer being able to show you historical
values. Doing so also breaks the notifications feature.

## CLI

A command line client is provided, implementing two subcommands:

* `discover`: uses multicast CoAP to find compatible devices on your network
* `publish`: publishes the data to MQTT
* `status`: like publish, but outputs on the CLI instead

## `philips`

The `philips` package contains all the logic to handle communication with
a device. It includes a lot of things to handle a ton of weird idiosyncracies
in both the protocol and its custom payload encryption scheme.

This package is usable without needing to be invested in the rest of the
Hemtjänst ecosystem.
