package main

import (
	"log"

	"github.com/aegis/aegis/internal/service"
)

func main() {
	if err := service.Run("aegis-metadata"); err != nil {
		log.Fatal(err)
	}
}
