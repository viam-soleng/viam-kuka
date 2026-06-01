# Viam KUKA Module

This is a [Viam module](https://docs.viam.com/build-modules/overview/) for [KUKA](https://www.kuka.com/en-us)'s family of industrial arms. The module provides a general framework for operating a compatible KUKA arm over a TCP connection using KUKA's EKI (Ethernet KRL Interface).

The module registers one model, `viam-soleng:arm:viam-kuka`, which implements the Viam [arm](https://docs.viam.com/components/arm/) API.

## Set up the EKI program on your controller

This module is the client side of a KUKA EKI connection. It does not drive the arm directly. Instead it sends commands over TCP to an EKI server program that must already be running on the KUKA controller.

The matching server program is included in this repository under [`src/ekimanager/`](src/ekimanager/):

- [`ekiMain.src`](src/ekimanager/ekiMain.src) - the main motion program that executes point-to-point moves and stop requests.
- [`ekiCommHandler.sub`](src/ekimanager/ekiCommHandler.sub) - the submit interpreter program that parses incoming commands and answers status and information requests.
- [`ekiConnectionMonitor.sub`](src/ekimanager/ekiConnectionMonitor.sub) - reopens the EKI channel when a client disconnects.
- [`ekiUtils.src`](src/ekimanager/ekiUtils.src) - shared parsing and response helpers.
- [`ekiGlobals.dat`](src/ekimanager/ekiGlobals.dat) - global variables and command keyword definitions.

Load these onto the controller and start the program before configuring the module. The module checks the program state when it starts and before every motion command (when `safe_mode` is enabled), and it will refuse to run unless the program reports a `Running` state.

## Configure your KUKA arm

[Add a machine](https://docs.viam.com/set-up-a-machine/first-machine/) in the Viam app. Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com/). Click the **+** icon next to your machine part in the left-hand menu and select **Configuration Block**. Search for `viam-kuka` and select the `viam-soleng:arm:viam-kuka` model. Click **Add to machine**, then enter a name and click **Add to machine**.

On the new component panel, copy and paste the following attribute template into your arm's attributes field:

```json
{
  "ip_address": "192.168.1.10",
  "port": 54610,
  "model": "KR10r900",
  "safe_mode": true,
  "joint_speed": 10
}
```

Edit the attributes as applicable, then save your configuration.

## Attributes

The following attributes are available:

| Name | Type | Inclusion | Description |
| ---- | ---- | --------- | ----------- |
| `ip_address` | string | **Required** | The IP address of the KUKA controller. |
| `port` | int | Optional | The port used for the TCP connection to the controller. The default is `54610`. |
| `model` | string | Optional | The KUKA arm model. This selects the URDF file that supplies the arm's kinematic and geometric data. Currently only `KR10r900` is supported, which is also the default. |
| `joint_speed` | float64 | Optional | The joint speed as a percentage of the arm's maximum, from `0` to `100`. The default is `6.28`. |
| `safe_mode` | bool | Optional | If `true`, the module checks that the EKI program on the controller is in the `Running` state before sending any motion command. The default is `false`. |

## Known supported hardware

Support for the following arms has been confirmed. Additional arms that operate with KUKA's Robot Language (KRL) can be supported given the proper URDF file.

| Device      | macOS | Linux |
|-------------|-------|-------|
| KR10 R900-2 |   X   |   X   |

## Next steps

- Learn more about [controlling your arm with Viam](https://docs.viam.com/motion-planning/move-an-arm/overview/).

## Get help

For any questions please reach out to us on our [Discord channel](https://discord.com/channels/1083489952408539288).
