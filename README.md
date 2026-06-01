# [Viam KUKA module](https://app.viam.com/module/viam-soleng/viam-kuka)

This is a [Viam module](https://docs.viam.com/build-modules/overview/) for [KUKA](https://www.kuka.com/en-us)'s family of industrial arms. The module provides a general framework for operating a compatible KUKA arm over a TCP connection using KUKA's EKI (Ethernet KRL Interface).

The module registers one model, `viam-soleng:arm:viam-kuka`, which implements the Viam [arm](https://docs.viam.com/components/arm/) API.

## Contents

- [Set up the EKI program on your controller](#set-up-the-eki-program-on-your-controller)
- [Configure your KUKA arm](#configure-your-kuka-arm)
- [Attributes](#attributes)
- [DoCommand](#docommand)
- [Interacting with the arm](#interacting-with-the-arm)
- [Known supported hardware](#known-supported-hardware)
- [Troubleshooting](#troubleshooting)
- [Build and develop](#build-and-develop)
- [Get help](#get-help)

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

[Add a machine](https://docs.viam.com/set-up-a-machine/first-machine/) in the Viam app. Navigate to the **CONFIGURE** tab of your machine's page in [the Viam app](https://app.viam.com/). Click the **+** icon next to your machine part in the left-hand menu and select **Configuration Block**. Search for `viam-kuka` and select the `viam-kuka/viam-kuka` result tagged **ARM** (owned by `viam-soleng`). Click **Add to machine**, then enter a name and click **Add to machine**.

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

> [!NOTE]
> `safe_mode` is `false` by default. Set it to `true` to have the module confirm the EKI program is in the `Running` state before each motion command.

## Attributes

The following attributes are available:

| Name | Type | Inclusion | Default | Description |
| ---- | ---- | --------- | ------- | ----------- |
| `ip_address` | string | **Required** | — | The IP address of the KUKA controller. |
| `port` | int | Optional | `54610` | The port used for the TCP connection to the controller. |
| `model` | string | Optional | `KR10r900` | The KUKA arm model. This selects the URDF file that supplies the arm's kinematic and geometric data. Currently only `KR10r900` is supported. |
| `joint_speed` | float64 | Optional | `6.28` | The joint speed as a percentage of the arm's maximum, from `0` to `100`. |
| `safe_mode` | bool | Optional | `false` | If `true`, the module checks that the EKI program on the controller is in the `Running` state before sending any motion command. |

## DoCommand

The arm implements [`DoCommand`](https://docs.viam.com/dev/reference/apis/components/arm/#docommand) as a passthrough for raw EKI commands. It reads the `cmd` key as a string and writes it verbatim to the controller over the TCP connection. It does not append a terminator (include any character the program expects, such as `;`), and it does not wait for or return the controller's response.

```json
{
  "cmd": "getcurrentjoints;"
}
```

Use this only if you know the EKI command syntax expected by the program running on your controller. For normal operation, use the standard arm API methods instead.

## Interacting with the arm

Once your machine shows as **Live** in the Viam app, you can:

- View live data and move the arm from the **CONTROL** tab. See [Control machines](https://docs.viam.com/fleet/control/).
- Control the arm programmatically with one of [Viam's client SDKs](https://docs.viam.com/dev/reference/sdks/).

## Known supported hardware

Support for the following arms has been confirmed. Additional arms that operate with KUKA's Robot Language (KRL) can be supported given the proper URDF file.

| Device      | macOS | Linux |
|-------------|-------|-------|
| KR10 R900-2 |   X   |   X   |

## Troubleshooting

If the module fails to start or a command is rejected, check the `viam-server` logs for the error text below.

| Error | Likely cause and fix |
| ----- | -------------------- |
| `associated program on your kuka device is ..., please get the program running before continuing` | The EKI program in `src/ekimanager/` is not in the `Running` state on the controller. Load and start it, then confirm it reports `Running`. |
| `robot is still moving, please try again after previous movement is complete` | A motion command was sent before the previous one finished. Wait for the current move to complete (or call `Stop`), then retry. |
| `invalid joint position specified, ... is outside of joint[N] limits` | The commanded angle for joint N is outside the limits the controller reported. Command a position within range. |
| `given model (...) not in list of supported models` | The `model` attribute is set to an unsupported value. Use `KR10r900` (the only supported model) or omit it. |
| Connection or dial errors on startup | `viam-server` cannot reach the controller at `ip_address:port`. Check the address, the port (default `54610`), and network connectivity to the controller. |

## Build and develop

This module is written in Go. Common Makefile targets:

- `make build` compiles the binary to `bin/viam-kuka-module`.
- `make test` runs the test suite.
- `make module` builds the packaged `module.tar.gz` AppImage used for registry deployment (built for `linux/arm64`).

## Get help

For any questions please reach out to us on our [Discord channel](https://discord.com/channels/1083489952408539288).

You can learn more about [controlling an arm with Viam](https://docs.viam.com/motion-planning/move-an-arm/overview/) in the Viam docs.
