package process

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/service"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)

type Process struct {
	db *db.Db
	sm service.ServerManager
}

// NewProcess creates a new Process
func NewProcess(d *db.Db, sm service.ServerManager) *Process {
	return &Process{
		db: d,
		sm: sm,
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
		err = p.sm.GetStreamFromServer(ctx, data.Value)
		if err != nil {
			log.Printf("error when resuming streaming: %v", err)
		}
	}()

	//d.ac

	return err
}
