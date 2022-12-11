# screensaver

A simple Linux screensaver framework

## Prerequisites

* You will need a X11 based Linux distribution to run the launcher.
    * The launcher does not support [Wayland](https://wayland.freedesktop.org/) yet.
* You will need a working Go compiler and Ebiten Linux packages installing.
    * https://go.dev
    * https://ebitengine.org/en/documents/install.html?os=linux

## How it works

This framework is composed of two parts, a launcher and a set of savers (coming
soon). The launcher is responsible for the launching of the savers, while the
savers themselves just draw something nice on-screen and exit when input is
detected.

The launcher uses a timer, that when expired, will launch the defined
executable. Any input will reset the timer and delay the launch.

## Install

Just run the `install.sh` script. This will compile and install the launcher and
savers.

## Add to startup

It differs between different Linux distributions but just add the launcher to
startup with the necessary options. (You must supply a `timer` and a `saver`
option.)

#### Example

```
screensaver-launcher -timer 15m -saver screensaver-game-of-life > /dev/null 2>&1
```

In this example you can see the `screensaver-game-of-life` saver is being
executed after the timer expires but any executable can be launched.

## Savers

### `screensaver-digital-rain`

![](screen/saver/digital_rain/assets/preview.gif)

### `screensaver-game-of-life`

![](screen/saver/game_of_life/assets/preview.gif)
