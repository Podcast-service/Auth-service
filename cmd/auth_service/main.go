package main

import (
	"log"

	"github.com/Podcast-service/Auth-service/internal/application/runner"
)

func main() {
	err := runner.Run()
	if err != nil {
		log.Fatal(err)
	}
}
