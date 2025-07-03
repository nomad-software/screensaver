# screensaver

A simple Linux screensaver framework

## Prerequisites

* This is a Linux only screensaver framework.
* To run the launcher you will need to use [Gnome](https://www.gnome.org/) as your desktop environment.
    * This is because the input detection is currently handled by [Mutter](https://mutter.gnome.org/).
    * [X11](https://www.x.org/wiki/) and [Wayland](https://wayland.freedesktop.org/) are supported.
* A working Go and C compiler and installed Raylib dependencies.
    * https://go.dev
    * https://github.com/gen2brain/raylib-go#requirements

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

Use [Tweaks](https://wiki.gnome.org/Apps/Tweaks) to add to start-up applications.

### Application file

You may need to create a desktop launcher to be used with newer version of Tweaks.
Create a file containing the following and place in `~/local/share/applications`.

```
[Desktop Entry]
Type=Application
Name=Screensaver Launcher
Exec=screensaver-launcher -saver screensaver-digital-rain -timer 15m
```

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

![](screen/saver/digital_rain/assets/images/preview.gif)

### `screensaver-game-of-life`

![](screen/saver/game_of_life/assets/images/preview.gif)
