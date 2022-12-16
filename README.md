# screensaver

A simple Linux screensaver framework

## Prerequisites

* A [X11](https://www.x.org/wiki/) based Linux distribution to run the launcher.
    * The launcher does not support [Wayland](https://wayland.freedesktop.org/) yet.
* A working Go and C compiler and installed Ebiten Linux dependencies.
    * https://go.dev
    * https://ebitengine.org/en/documents/install.html?os=linux

## How it works

This framework is composed of multiple parts, a launcher, multiple savers and a
command tool to send commands to the launcher. The launcher is responsible for
the launching of the savers, while the savers themselves just draw something
nice on-screen and exit when input is detected.

The launcher uses a timer, that when expired, will launch the defined
executable. Any input will reset the timer and delay the launch.

## Install

Just run the `install.sh` script. This will compile and install the various binaries.

## Add to startup

It differs between different Linux distributions but just add the launcher to
startup with the necessary options.

### Example

```
screensaver-launcher -timer 15m -saver screensaver-game-of-life > /dev/null 2>&1
```

In this example you can see the `screensaver-game-of-life` saver is being
executed after the timer expires but any executable can be launched.

## Commands

You can use the command tool to send commands to the launcher to control its behaviour.

| Command                         | Description                                         |
|---------------------------------|-----------------------------------------------------|
| `screensaver-command -reset`    | Reset the launcher timer.                           |
| `screensaver-command -activate` | Expires the timer and activate the specified saver. |

## Savers

### `screensaver-digital-rain`

![](screen/saver/digital_rain/assets/preview.gif)

### `screensaver-game-of-life`

![](screen/saver/game_of_life/assets/preview.gif)
