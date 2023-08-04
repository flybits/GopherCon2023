package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"github.com/flybits/gophercon2023/client/handler"
	"github.com/flybits/gophercon2023/client/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	var err error

	err = setRabbitCreds()
	if err != nil {
		log.Printf("error setting rabbit credentials: %v", err)
	}

	//p := process.NewProcess()
	broker := amqp.Broker{}
	err = broker.SetupBroker([]amqp.Exchange{
		amqp.ExchangeWithDefaults("events-listener", ""),
	}, []amqp.Queue{
		/*{
			Name:       "client",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Bindings: []amqp.Binding{
				amqp.BindingWithDefaults("oom", "client"),
			},
			Consumers: []amqp.Consumer{
				amqp.ConsumerWithDefaults(false, p.ProcessAMQPMsg),
			},
		},*/
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

	/*
			kubeconfig := `apiVersion: v1
		kind: Config
		clusters:
		- name: "eks-development"
		  cluster:
		    server: "https://new-rancher.flybits.tools/k8s/clusters/c-m-ncwjc5pj"
		users:
		- name: "eks-development"
		  user:
		    token: "kubeconfig-u-6kt6lzlg2nwx6nq:xc987k72s9px4dptvsvjls64xzhcblqj6mlclwgvbrkwdlkppmhdd7"
		contexts:
		- name: "eks-development"
		  context:
		    user: "eks-development"
		    cluster: "eks-development"
		    namespace: "development"
		current-context: "eks-development"
		`
			client, err := k8sutils.NewKubeClient(kubeconfig)
			if err != nil {
				klog.Fatal(err)
			}


	*/

	// creates the in-cluster config
	conf, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		panic(err.Error())
	}

	eventCh := make(chan *watcher.ContainerRestartEvent)
	stopCh := make(chan struct{})

	listen(clientset, eventCh, stopCh)
	go processRestartEvents(eventCh, &broker)
	/*
		time.Sleep(10 * time.Second)

		// get pods in all the namespaces by omitting namespace
		// Or specify namespace to get pods in particular namespace
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		// Examples for error handling:
		// - Use helper functions e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		_, err = clientset.CoreV1().Pods("default").Get(context.TODO(), "example-xxxxx", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod example-xxxxx not found in default namespace\n")
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found example-xxxxx pod in default namespace\n")
		}

		time.Sleep(10 * time.Second)
	*/
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

	log.Printf("Exiting...")
}

func listen(client *kubernetes.Clientset, eventCh chan *watcher.ContainerRestartEvent, stopCh chan struct{}) {
	watcher := watcher.NewPodWatcher(client, eventCh)

	err := watcher.Run(stopCh)
	if err != nil {
		log.Printf(" error in listen %v", err)
	}
}

func processRestartEvents(eventCh chan *watcher.ContainerRestartEvent, broker *amqp.Broker) {
	for event := range eventCh {
		log.Printf("event from k8s: %+v", event)
		if strings.Contains(event.Reason, "OOM") {
			b, err := json.Marshal(event)
			if err != nil {
				log.Printf("error marshaling event: %v this event %v", err, event)
			}
			publish := amqp.PublishWithDefaults("events-listener", "oom", b)
			err = broker.Publish(context.Background(), publish)
			if err != nil {
				log.Printf("error publishing oom event %v", err)
			}
		}
	}
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
