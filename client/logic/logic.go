package logic

import (
	"github.com/flybits/gophercon2023/client/service"
	"sync"
)

type Controller struct {
	ServerManager        service.ServerManager
	StreamStartedChannel chan bool
	InterruptionChannel  chan bool
	WaitGroup            sync.WaitGroup
}

func NewController(sm service.ServerManager) Controller {
	streamCh := make(chan bool)
	interruptionCh := make(chan bool)

	return Controller{
		ServerManager:        sm,
		StreamStartedChannel: streamCh,
		InterruptionChannel:  interruptionCh,
	}
}
