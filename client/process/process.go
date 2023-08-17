package process

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/logic"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)

type Process struct {
	db         *db.Db
	controller *logic.Controller
}

// NewProcess creates a new Process
func NewProcess(d *db.Db, c *logic.Controller) *Process {
	return &Process{
		db:         d,
		controller: c,
	}
}

type containerRestartEvent struct {
	Namespace     string
	PodName       string
	ContainerName string
	RestartCount  int32
	ExitCode      int32
	Reason        string
	Message       string
}

func (p *Process) ProcessAMQPMsg(ctx context.Context, d amqp.Delivery) error {
	log.Printf("received message %v", string(d.Body))

	var err error
	switch d.RoutingKey {
	case "interrupted":
		err = p.processInterrupted(ctx, d)
	case "oom":
		err = p.processOOM(ctx, d)
	}

	if err != nil {
		log.Printf("error processing event: %v", err)
	}

	err = d.Ack(false)
	if err != nil {
		log.Printf("failed to acknowledge message")
	}
	return err
}

func (p *Process) processInterrupted(ctx context.Context, d amqp.Delivery) error {
	log.Printf("received message for interruption %v", string(d.Body))
	var msg logic.InterruptionMessage
	err := json.Unmarshal(d.Body, &msg)
	if err != nil {
		return err
	}

	// continuing processing interruption in a go routine to allow the message to be acknowledged fast
	// so that on graceful shutdown we can close consuming on the queue before sending new interruption messages
	// this is to ensure the same terminating pod does not pick up the newly emitted interruption message
	go func() {
		err = p.controller.CarryOnInterruptedStreaming(ctx, msg)
		if err != nil {
			log.Printf("error when carrying on streaming")
		}
	}()
	return nil
}

func (p *Process) processOOM(ctx context.Context, d amqp.Delivery) error {
	log.Printf("received message for OOM %v", string(d.Body))

	var e containerRestartEvent
	err := json.Unmarshal(d.Body, &e)
	if err != nil {
		log.Printf("error unmarshaling event: %v", err)
	}
	stm, err := p.db.GetOngoingStreamWithPodName(ctx, e.PodName)
	if errors.Is(err, mongo.ErrNoDocuments) {
		// nothing to be done
		log.Printf("nothing to be done...pod %v did not have an ongoing streaming", e.PodName)
		return nil
	}

	if err != nil {
		log.Printf("error retrieving streaming with pod name: %v", err)
	}

	data, err := p.db.GetPointOfInterruption(ctx, stm.ID)
	if err != nil {
		log.Printf("error when getting point of interruption: %v", err)
		return err
	}

	go func() {
		err = p.controller.PerformStreaming(ctx, data.Value)
		if err != nil {
			log.Printf("error when resuming streaming: %v", err)
		}
	}()

	return err
}
