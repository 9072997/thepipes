package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	// settings from the installer are available as environment variables
	listenAddr := os.Getenv("listen")

	// flags must be configured via AppManifest.Command
	name := flag.String("flag", "Default Value", "Example of a command-line flag")
	flag.Parse()

	// The working directory is our data directory. We can write data here.
	// Additionally there is a JSON file with all the install config values
	// (this is just another alternative to environment variables)

	http.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Hello from the payload server!\n\n")
		fmt.Fprintf(w, "Install dir: %s\n", os.Getenv("INSTALL"))
		fmt.Fprintf(w, "Data dir: %s\n", os.Getenv("DATA"))
		fmt.Fprintf(w, "Service account: %s\n", os.Getenv("SERVICE_ACCOUNT"))
		fmt.Fprintf(w, "Auto-updates enabled: %s\n", os.Getenv("AUTO_UPDATES"))
		fmt.Fprintf(w, "Flag value: %s\n", *name)

		wd, _ := os.Getwd()
		fmt.Fprintf(w, "Current working directory %s:\n", wd)
		files, _ := os.ReadDir(".")
		for _, f := range files {
			fmt.Fprintf(w, "- %s\n", f.Name())
		}
	}))

	srv := &http.Server{}

	// ThePipes sends CTRL_BREAK_EVENT when the SCM requests a stop, then waits
	// 10 seconds before killing the process.  Go's runtime translates both
	// Ctrl+C and Ctrl+Break to os.Interrupt, so one handler covers both.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Println("INFO: shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		fmt.Println("INFO: shutdown complete")
	}()

	// we can generate event log entries by printing lines with special prefixes
	fmt.Printf("INFO: starting server on %s\n", listenAddr)
	err := srv.ListenAndServe()
	if err != http.ErrServerClosed {
		fmt.Printf("ERROR: server error: %v\n", err)
		os.Exit(1)
	}
}
