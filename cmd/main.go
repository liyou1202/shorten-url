package main

import (
	"log"
	"net/http"
	"os"

	cloudFunction "liyou-chen.com/shorten-url"
)

func main() {
	http.HandleFunc("/", cloudFunction.RequestHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
