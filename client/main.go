package main

import (
	"context"
	"fmt"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/handler"
	"github.com/flybits/gophercon2023/client/logic"
	"github.com/flybits/gophercon2023/client/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var err error

	database, err := db.NewMongoDb()
	if err != nil {
		log.Printf("failed to connect to mongodb: %v", err)
	}

	podName := os.Getenv("CONFIG_POD_NAME")
	log.Printf("the pod name is %v", podName)

	broker := amqp.Broker{}
	sm, err := service.NewServerManager("server:8001")
	if err != nil {
		log.Printf("error when connecting to server grpc:%v", err)
	}

	delay := os.Getenv("CONFIG_DELAY")
	log.Printf("delay is %v", delay)
	d, err := time.ParseDuration(delay)
	if err != nil {
		log.Printf("error parsing duration %v", err)
	}

	config.Global.Delay = d

	c := logic.NewController(sm, &broker, database)

	log.Printf("Starting HTTP server ...")

	h := handler.NewHandler(&c)
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
	shutdownGracefully(ctx, httpServer, &broker, &c)
	log.Printf("Exiting...")
}

func shutdownGracefully(ctx context.Context, httpServer *http.Server, broker *amqp.Broker, controller *logic.Controller) {

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("failed to gracefully shut down HTTP server: %s", err.Error())
	} else {
		log.Printf("Successfully shut down http server gracefully")
	}

}
