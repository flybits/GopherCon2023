package main

import (
	"context"
	"fmt"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/handler"
	"github.com/flybits/gophercon2023/client/logic"
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

	c := logic.NewController(sm, &broker, database)

	p := process.NewProcess(database, &c)
	err = broker.SetupBroker([]amqp.Exchange{
		amqp.ExchangeWithDefaults("events-listener", ""),
		amqp.ExchangeWithDefaults("client", ""),
	}, []amqp.Queue{
		{
			Name:       "oom",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Bindings: []amqp.Binding{
				amqp.BindingWithDefaults("oom", "events-listener"),
			},
			Consumers: []amqp.Consumer{
				amqp.ConsumerWithDefaults(false, p.ProcessAMQPMsg),
			},
		},
		{
			Name:       "interrupted",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Bindings: []amqp.Binding{
				amqp.BindingWithDefaults("interrupted", "client"),
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
		panic("error connecting to rabbitmq")
	} else {
		log.Println("connected to rabbitmq server")
	}

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

func shutdownGracefully(ctx context.Context, httpServer *http.Server, broker *amqp.Broker, controller *logic.Controller) {

	errs := broker.ShutDownConsumersForQueues(ctx, []string{"interrupted"})
	if errs == nil {
		log.Printf("successfully shut down rabbitmq consumers for specific queues")
	} else {
		log.Printf("the following errors happened when shutting down specific queues: %v", errs)
	}

	// a loop here is appropriate because there could be multiple go routines that need to be gracefully interrupted
	breakOut := false
	for {
		select {
		case <-controller.StreamStartedChannel:
			controller.WaitGroup.Add(1)
			controller.InterruptionChannel <- true
			log.Printf("interruption message sent")
		default:
			breakOut = true
			break
		}
		if breakOut {
			log.Printf("finished gracefully interrupting streaming")
			break
		}
	}

	controller.WaitGroup.Wait()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("failed to gracefully shut down HTTP server: %s", err.Error())
	} else {
		log.Printf("Successfully shut down http server gracefully")
	}

	if err := broker.ShutDown(ctx); err != nil {
		log.Printf("failed to gracefully shut down rabbitMQ broker: %s", err.Error())
	} else {
		log.Printf("Successfully shut down rabbitMQ broker gracefully")
	}
}
