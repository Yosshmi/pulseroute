package main

import (
	"log"
	"pulseroute/internal/app"
)

func main() {
	if e := app.Run("api"); e != nil {
		log.Fatal(e)
	}
}
