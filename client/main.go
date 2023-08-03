package main

import (
	"context"
	"fmt"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"github.com/flybits/gophercon2023/client/db"
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

	err = setRabbitCreds()
	if err != nil {
		log.Printf("error setting rabbit credentials: %v", err)
	}
	p := process.NewProcess()
	broker := amqp.Broker{}
	err = broker.SetupBroker([]amqp.Exchange{
		amqp.ExchangeWithDefaults("client", ""),
	}, []amqp.Queue{
		{
			Name:       "client",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Bindings: []amqp.Binding{
				amqp.BindingWithDefaults("routingKey", "client"),
			},
			Consumers: []amqp.Consumer{
				amqp.ConsumerWithDefaults(false, p.ProcessAMQPMsg),
			},
		},
	},
		amqp.URIScheme(config.Global.RabbitmqScheme),
		amqp.Address(config.Global.RabbitmqAddress, config.Global.RabbitmqPort),
		amqp.Credentials(config.Global.RabbitmqUsername, config.Global.RabbitmqPassword),
		amqp.Vhost(config.Global.RabbitmqVhost))

	if err != nil {
		log.Printf("error when connecting to rabbitmq server: %v", err)
	} else {
		log.Println("connected to rabbitmq server")
	}

	podName := os.Getenv("CONFIG_POD_NAME")
	log.Printf("the pod name is %v", podName)

	db, err := db.NewMongoDb()
	if err != nil {
		log.Printf("failed to connect to mongodb: %v", err)
	}

	sm, err := service.NewServerManager("server:8001", &broker, db)
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

func setRabbitCreds() error {
	passb, err := os.ReadFile("/etc/rabbitmq-admin/pass")
	if err != nil {
		return err
	}
	userb, err := os.ReadFile("/etc/rabbitmq-admin/user")
	if err != nil {
		return err
	}

	addressb, err := os.ReadFile("/etc/rabbitmq-admin/address")
	if err != nil {
		return err
	}
	config.Global.RabbitmqUsername = string(userb)
	config.Global.RabbitmqPassword = string(passb)
	config.Global.RabbitmqAddress = string(addressb)

	return nil
}
