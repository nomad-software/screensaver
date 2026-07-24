#!/bin/bash

# Install the lancher and tools.
go install cmd/launcher/screensaver-launcher.go
go install cmd/command/screensaver-command.go

# Install the savers.
go install screen/saver/connection_machine/screensaver-connection-machine.go;
go install screen/saver/digital_rain/screensaver-digital-rain.go;
go install screen/saver/game_of_life/screensaver-game-of-life.go;
