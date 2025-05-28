package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/0xdvc/go-paxos/server"
)

func main() {
	server := server.NewServer()
	webDir := "./web"

	absPath, _ := filepath.Abs(webDir)

	fs := http.FileServer(http.Dir(webDir))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fs.ServeHTTP(w, r)
	})

	http.HandleFunc("/api/state", server.HandleGetState)
	http.HandleFunc("/api/start", server.HandleStartSimulation)
	http.HandleFunc("/api/stop", server.HandleStopSimulation)
	http.HandleFunc("/api/options", server.HandleOptions)

	port := 3000
	address := fmt.Sprintf(":%d", port)

	fmt.Printf("starting paxos visualizer server at http://localhost%s\n", address)
	fmt.Printf("serving files from: %s\n", absPath)
	fmt.Println("press ctrl+c to stop the server")

	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal("Server error:", err)
	}
}
