#!/bin/bash

# Install the lancher.
go install screensaver-launcher.go

# Install the savers.
go install screen/saver/game_of_life/screensaver-game-of-life.go;
go install screen/saver/digital_rain/screensaver-digital-rain.go;
