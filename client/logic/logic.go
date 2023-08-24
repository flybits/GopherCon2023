package logic

import (
	"context"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/service"
	"github.com/flybits/gophercon2023/server/pb"
	"io"
	"log"
	"sync"
)

type Controller struct {
	ServerManager        service.ServerManager
	broker               *amqp.Broker
	db                   *db.Db
	StreamStartedChannel chan bool
	InterruptionChannel  chan bool
	WaitGroup            sync.WaitGroup
}

func NewController(sm service.ServerManager, b *amqp.Broker, d *db.Db) Controller {
	streamCh := make(chan bool, 100)
	interruptionCh := make(chan bool, 100)

	return Controller{
		ServerManager:        sm,
		broker:               b,
		db:                   d,
		StreamStartedChannel: streamCh,
		InterruptionChannel:  interruptionCh,
		WaitGroup:            sync.WaitGroup{},
	}
}

func (c *Controller) PerformStreaming(ctx context.Context, offset int32, streamID string) error {

	var sm db.StreamMetadata
	var err error

	log.Printf("starting streaming with id %v from offset: %v", streamID, offset)

	stream, err := c.ServerManager.GetStreamFromServer(ctx, offset)

	if err != nil {
		log.Printf("error getting stream: %v", err)
		return err
	}

	// a channel to receive errors from
	errCh := make(chan error)
	go c.receiveStream(ctx, stream, sm, sm.LastUserIDStreamed, errCh)

	// waiting to hear from the goroutine above
	err = <-errCh
	return err
}

func (c *Controller) receiveStream(ctx context.Context, stream pb.Server_GetDataClient, sm db.StreamMetadata, lastSuccessfullyProcessedUserID string, errCh chan error) {

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			log.Printf("received end of stream")
			break
		}

		if err != nil {
			errCh <- err
			return
		}

		log.Printf("received data: %v", data)

		err = c.processData(data, sm.ID)
		if err != nil {
			log.Printf("error when processing data: %v", err)
			continue
		}

		log.Printf("processed data %v", data)

	}

	errCh <- nil
}

func (c *Controller) processData(data *pb.Data, streamID string) error {
	err := c.db.UpsertData(context.Background(), data, streamID)
	return err
}
