package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type workerConfig struct {
	ScanReader map[string]any `toml:"scan_reader"`
	Advanced   struct {
		StatusAddress string `toml:"status_address"`
		StatusPort    int    `toml:"status_port"`
	} `toml:"advanced"`
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("redis-shake version fake test/worker (Git SHA: fake)")
		return
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "expected config path")
		os.Exit(2)
	}
	contents, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	config := workerConfig{}
	if err := toml.Unmarshal(contents, &config); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	address := config.Advanced.StatusAddress
	if address == "" {
		address = "127.0.0.1"
	}
	server := &http.Server{
		Addr: net.JoinHostPort(address, strconv.Itoa(config.Advanced.StatusPort)),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"start_time":          "2026-08-21 12:00:00",
				"consistent":          true,
				"total_entries_count": map[string]any{"read_count": 3, "write_count": 3},
				"reader":              map[string]any{"status": "running"},
				"writer":              map[string]any{"unanswered_entries": 0},
			})
		}),
		ReadHeaderTimeout: 2 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	fmt.Println("fake worker started password=worker-secret")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	if config.ScanReader != nil {
		select {
		case <-time.After(800 * time.Millisecond):
		case <-signals:
		}
	} else {
		select {
		case <-signals:
		case err := <-serverErrors:
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownContext)
	fmt.Println("fake worker stopped")
}
