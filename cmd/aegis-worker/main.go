package main

import (
	"log"

	"github.com/aegis/aegis/internal/service"
)

func main() {
	if err := service.Run("aegis-worker"); err != nil {
		log.Fatal(err)
	}
}
