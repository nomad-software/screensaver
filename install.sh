#!/bin/bash

# Install the lancher and tools.
go install cmd/launcher/screensaver-launcher.go
go install cmd/command/screensaver-command.go

# Install the savers.
go install screen/saver/connection_machine/screensaver-connection-machine.go;
go install screen/saver/digital_rain/screensaver-digital-rain.go;
go install screen/saver/game_of_life/screensaver-game-of-life.go;
go install screen/saver/shader_fire/screensaver-shader-fire.go;
go install screen/saver/shader_reef/screensaver-shader-reef.go;
go install screen/saver/shader_star_grid/screensaver-shader-star-grid.go;
