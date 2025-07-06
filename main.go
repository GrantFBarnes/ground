package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello")
	})

	port := ":3478"
	log.Printf("server is running on http://localhost%s...\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
}
