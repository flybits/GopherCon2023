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

	database, err := db.NewMongoDb()
	if err != nil {
		log.Printf("failed to connect to mongodb: %v", err)
	}

	podName := os.Getenv("CONFIG_POD_NAME")
	log.Printf("the pod name is %v", podName)

	broker := amqp.Broker{}
	sm, err := service.NewServerManager("server:8001", &broker, database)
	if err != nil {
		log.Printf("error when connecting to server grpc:%v", err)
	}

	p := process.NewProcess(database, sm)
	err = broker.SetupBroker([]amqp.Exchange{
		amqp.ExchangeWithDefaults("events-listener", ""),
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

/*
func shutdownGracefully(ctx context.Context, logger *log.Logger, httpServer *http.Server, broker *amqp.Broker, grpc *grpc.Server, controller *logic.Controller) {

	errs := broker.ShutDownConsumersForQueues(ctx, []string{"rulesInterruptedEvaluationForAll"})
	if errs == nil {
		logger.Info("successfully shut down rabbitmq consumers for specific queues")
	} else {
		logger.Errorf("the following errors happened when shutting down specific queues: %v", errs)
	}

	// a loop here is appropriate because there could be multiple go routines that need to be gracefully interrupted
	breakOut := false
	for {
		select {
		case <-controller.EvaluationForAllStartedChannel:
			controller.InterruptionChannel <- true
			logger.Info("interruption message sent")

			// wait for completion to finish
			<-controller.InterruptionCompleted
			logger.Info("interruption completed")
		default:
			breakOut = true
			break
		}
		if breakOut {
			logger.Info("finished gracefully interrupting long running tasks")
			break
		}
	}

	//	logger.Info("interruption completed (no info about success criteria)")
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Errorf("failed to gracefully shut down HTTP server: %s", err.Error())
	} else {
		logger.Info("Successfully shut down http server gracefully")
	}

	if err := broker.ShutDown(ctx); err != nil {
		logger.Errorf("failed to gracefully shut down rabbitMQ broker: %s", err.Error())
	} else {
		logger.Info("Successfully shut down rabbitMQ broker gracefully")
	}

	grpc.GracefulStop()
	logger.Info("Successfully shut down grpc server gracefully")
}
*/
