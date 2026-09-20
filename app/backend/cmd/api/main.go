package main

import (
	"log"
	"web-monitor/internal/api"
)

func main() {
	client := api.NewAPIClient()
	log.Fatal(client.Start("8080"))
}
