package process

import (
	"context"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/logic"
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

	return nil
}
