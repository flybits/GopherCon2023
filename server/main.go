package main

import (
	"context"
	"fmt"
	"github.com/flybits/gophercon2023/server/handler"
	"github.com/flybits/gophercon2023/server/logic"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var err error

	log.Printf("Initializing rpc server ...")
	grpc, err := logic.Setup(":8001")
	if err != nil {
		panic(fmt.Sprintf("Could not initiate a listen on rpc requests: %s", err))
	}

	log.Printf("Starting HTTP server ...")

	h := handler.NewHandler()
	router := handler.NewRouter(h)
	httpServer := &http.Server{
		Addr:      ":8000",
		Handler:   router,
		TLSConfig: nil,
	}
	// listen for sigint/term from OS to trigger graceful shut down
	terminationChannel := make(chan os.Signal, 1)
	signal.Notify(terminationChannel, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err = httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("Error starting HTTP server: %s", err))
		}
	}()

	sig := <-terminationChannel

	log.Printf("Termination signal '%s' received, initiating graceful shutdown...", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(25)*time.Second)
	defer cancel()

	// shutdown the http and grpc servers
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("failed to gracefully shut down HTTP server: %s", err.Error())
	} else {
		log.Printf("Successfully shut down http server gracefully.")
	}

	// Note: This may be a blocking call
	// https://godoc.org/google.golang.org/grpc#Server.GracefulStop
	grpc.GracefulStop()

	log.Printf("Exiting...")
}
