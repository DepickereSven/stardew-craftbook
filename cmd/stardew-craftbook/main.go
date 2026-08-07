// Command stardew-craftbook serves the craftbook UI and JSON API for a local
// Stardew Valley save, over the LAN so a phone can reach it.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/svendep/stardew-craftbook/internal/parser"
	"github.com/svendep/stardew-craftbook/internal/server"
)

func main() {
	savePath := flag.String("save-path", "", "explicit path to a save file (overrides detection)")
	saveName := flag.String("save-name", "", "save folder name to pick within the detected root")
	port := flag.Int("port", 8375, "HTTP port")
	pollSec := flag.Int("poll", 3, "save poll interval, seconds")
	flag.Parse()

	// Per spec §8 the server starts even without a save; /api/state then
	// carries the detection error so the UI can display it.
	path, detectErr := *savePath, ""
	if path == "" {
		roots := parser.CandidateRoots()
		for _, root := range roots {
			if p, err := parser.FindSave(root, *saveName); err == nil {
				path = p
				break
			}
		}
		if path == "" {
			detectErr = fmt.Sprintf("no save found; tried roots %v — use --save-path to point at a save file", roots)
			log.Print(detectErr)
		}
	}
	if path != "" {
		log.Printf("using save: %s", path)
	}

	srv, err := server.New(path, detectErr)
	if err != nil {
		log.Fatal(err)
	}
	if path != "" {
		srv.StartPolling(time.Duration(*pollSec) * time.Second)
	}
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("serving on http://0.0.0.0%s (open from your phone via this machine's LAN IP)", addr)
	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
}
