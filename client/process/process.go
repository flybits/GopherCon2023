package process

import (
	"context"
	"github.com/flybits/gophercon2023/amqp"
	"log"
)

type Process struct {
}

// NewProcess creates a new Process
func NewProcess() *Process {
	return &Process{}
}

func (p *Process) ProcessAMQPMsg(ctx context.Context, d amqp.Delivery) (err error) {
	log.Printf("received message %v", string(d.Body))
	return err
}
