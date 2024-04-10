# Viam KUKA Module

This is a [Viam module](https://docs.viam.com/manage/configuration/#modules) for [KUKA](https://www.kuka.com/en-us)'s family of industrial arms. This module will create a TCP client connection to a Kuka controller running Kuka's EKI Manager software in their SmartHMI.


## Configure your KUKA Arm

After creating a new arm component resource and adding this module to your config, several attributes can be added to specify certain configurations of your arm:


```json
{
  "ip_address": "0.0.0.0",
  "port": 1234,
  "model": "S1",
  "safe_mode": true,
  "joint_speed": 10
}
```

Edit the attributes as applicable.

## Attributes

The following attributes are available:

| Name | Type | Inclusion | Description |
| ---- | ---- | --------- | ----------- |
| `ip_address` | string | Required | The IP address of the KUKA device.  |
| `port` | int | Optional | The port on the device to form the required TCP connection. The default port is 54610.  |
| `model` | string | Optional | The baudrate model of KUKA device to be communicated to. This is also used in order to load the proper URDF file for geometric and kinematic data. The default model is KR10 R900-2.  |
| `joint_speed` | float64 | Optional | Sets the speed of the joints. A value from (1-100). The default speed is 6.28  |
| `safe_mode` | bool | Optional | A bool that, if true, will ping the KUKA device to check connection before running any motion actions. The default is safe_mode turned off. |

## Known Supported Hardware

Support for the following Arms has been confirmed. Additional arms that operate via KUKA's Robot Language (KRL) can be supported given the proper URDF file.

| Devices             | Mac OSX |  Linux  |
|---------------------|---------|---------|
| KR10r900-2          |    X    |    X    | 

## Further Work

To request additional features or models be added, please create a GitHub Issue or reach out to us on our [Discord channel](https://discord.com/channels/1083489952408539288). 
