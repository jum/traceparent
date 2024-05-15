# traceparent

The traceparent package contains a simple middleware for the [slogdriver](https://github.com/jussi-kalliokoski/slogdriver) package. Slogdriver does already contain all the machinery to output the proper trace and span id's if they are present in the context used for an slog call. It leaves the parsing of traceparent headers to some of the bigger packages like OTEL. In the context of Google Cloud Logging, the full OTEL packages might not be needed, as Google Cloud Logging does display all traces and spans in context when parsing the structured log entries. This small middleware fills the gap.

An example how to use this in a go app:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jussi-kalliokoski/slogdriver"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	debug := os.Getenv("NODE_ENV") == "development"
	level := new(slog.LevelVar) // Info by default
	if debug {
		level.Set(slog.LevelDebug)
	}
	var shandler slog.Handler
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if len(projectID) > 0 {
		shandler = slogdriver.NewHandler(os.Stderr, slogdriver.Config{
			Level:     level,
			ProjectID: projectID,
		})
	} else {
		shandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	}
	logger := slog.New(shandler)
	slog.SetDefault(logger)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8089"
		slog.Debug("Defaulting", "port", port)
	}
	var network string
	var addr string
	if strings.HasPrefix(port, "/") {
		network = "unix"
		addr = port
		err := os.Remove(addr)
		if err != nil && !os.IsNotExist(err) {
			slog.Error("remove unix socket", "err", err)
		}
		defer os.Remove(addr)
		slog.Info("Listening", "addr", addr)
	} else {
		network = "tcp"
		addr = fmt.Sprintf(":%s", port)
		// Output somthing to cmd-click on
		slog.Info("Listening", "port", port, "url", fmt.Sprintf("http://localhost:%s/", port))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.DebugContext(ctx, "incoming", "parent", r.Header.Get("traceparent"))
		fmt.Fprintf(w, "Hello, World!\n")
	})
	h2s := &http2.Server{}
	srv := http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(Traceparent(mux), h2s),
	}
	listener, err := net.Listen(network, addr)
	if err != nil {
		slog.Error("init listen", "err", err)
		os.Exit(1)
	}
	if network == "unix" {
		err := os.Chmod(addr, 0666)
		if err != nil {
			slog.Error("chmod", "addr", addr, "err", err)
		}
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		if err := srv.Serve(listener); err != nil {
			if err != http.ErrServerClosed {
				slog.Error("Serve", "err", err)
			}
		}
	}()
	<-stop
	slog.Debug("Shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Shutdown", "err", err)
	}
}
```

The environment variable GOOGLE_CLOUD_PROJECT needs to be set to
your google project id, it is used to format a proper trace reference for Google Cloud Logging.
