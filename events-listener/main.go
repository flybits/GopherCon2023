package main

import (
	"context"
	"fmt"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"github.com/flybits/gophercon2023/client/handler"
	"github.com/flybits/gophercon2023/client/process"
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

	p := process.NewProcess()
	broker := process.Broker{}
	err = broker.SetupBroker([]process.Exchange{
		process.ExchangeWithDefaults("client", ""),
	}, []process.Queue{
		{
			Name:       "client",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Bindings: []process.Binding{
				process.BindingWithDefaults("routingKey", "client"),
			},
			Consumers: []process.Consumer{
				process.ConsumerWithDefaults(false, p.ProcessAMQPMsg),
			},
		},
	},
		process.URIScheme(config.Global.RabbitmqScheme),
		process.Address(config.Global.RabbitmqAddress, config.Global.RabbitmqPort),
		process.Credentials(config.Global.RabbitmqUsername, config.Global.RabbitmqPassword),
		process.Vhost(config.Global.RabbitmqVhost))

	if err != nil {
		log.Printf("error when connecting to rabbitmq server: %v", err)
	} else {
		log.Println("connected to rabbitmq server")
	}

	sm, err := service.NewServerManager("server:8001", &broker)
	if err != nil {
		log.Printf("error when connecting to server grpc:%v", err)
	}
	log.Printf("Starting HTTP server ...")

	h := handler.NewHandler(sm)
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

	log.Printf("Exiting...")
}
