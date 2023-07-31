package process

import (
	"context"
	"log"
)

type Process struct {
}

// NewProcess creates a new Process
func NewProcess() *Process {
	return &Process{}
}

func (p *Process) ProcessAMQPMsg(ctx context.Context, d Delivery) (err error) {
	log.Printf("received message %v", string(d.Body))
	return err
}
