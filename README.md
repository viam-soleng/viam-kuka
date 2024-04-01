# Viam KUKA Module

This is a [Viam module](https://docs.viam.com/manage/configuration/#modules) for [KUKA](https://www.kuka.com/en-us)'s family of arms.


## Configure your KUKA Arm

After creating a new arm component resource and adding this module to your config, several attributes can be added to specify certain configurations of your arm:


```json
{
  "model": "S1",
  "ip_address": "0.0.0.0",
  "port": 1234,
  "safe_mode": true
}
```

Edit the attributes as applicable.

## Attributes

The following attributes are available:

| Name | Type | Inclusion | Description |
| ---- | ---- | --------- | ----------- |
| `ip_address` | string | Optional | The IP address of the KUKA device.  |
| `port` | int | Optional | The port on the device to form the required TCP connection. The default port is 54610.  |
| `model` | string | Optional | The baudrate model of KUKA device to be communicated to. This is also used in order to load the proper URDF file for geometric and kinematic data.  |
| `safe_mode` | bool | Optional | A bool that, if true, will ping the KUKA device to check connection before running any motion actions. |

## Known Supported Hardware

Support for the following Arms has been confirmed. Additional arms that operate via KUKA's Robot Language (KRL) should also be supported given the proper URDF file.

| Devices             | Mac OSX |  Linux  |
|---------------------|---------|---------|
| KR10r900            |    X    |    X    | 
