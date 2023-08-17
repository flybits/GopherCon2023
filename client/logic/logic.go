package logic

import (
	"context"
	"encoding/json"
	"fmt"
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

func (c *Controller) PerformStreaming(ctx context.Context, offset int32) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic at offset %v: panic %v", offset, r)

			go func() {
				offset++
				log.Printf("will continue processing data from offset %d\n", offset)

				err := c.PerformStreaming(ctx, offset)
				if err != nil {
					log.Printf("error when performing streaming: %v", err)
				}
			}()
		}
	}()

	// mark the start of streaming
	c.StreamStartedChannel <- true

	log.Printf("starting streaming from offset: %v", offset)

	sm, err := c.db.UpsertStreamMetadata(ctx, db.StreamMetadata{
		Offset: offset,
	})

	if err != nil {
		log.Printf("error inserting stream metadata")
	}

	log.Printf("streamID is %v", sm.ID)

	stream, err := c.ServerManager.GetStreamFromServer(ctx, offset)

	if err != nil {
		log.Printf("error getting stream: %v", err)
		return err
	}

	var lastSuccessfullyProcessedUserID string
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Printf("Err var returned from stream Recv: %v", err)
			return c.ShutDownStreamingGracefully(ctx, sm, lastSuccessfullyProcessedUserID, false)
		}

		log.Printf("received data: %v", data)

		// keeping the offset to the latest value we've received
		offset = data.Value

		err = c.processData(data, sm.ID)
		if err != nil {
			log.Printf("error when processing data: %v", err)
			continue
		}

		log.Printf("processed data %v", data)

		// listen to graceful shutdown messages in case you need to interrupt the process in middle gracefully
		select {
		case <-c.InterruptionChannel:
			log.Printf("going to interrupt streaming...")
			return c.ShutDownStreamingGracefully(ctx, sm, data.UserID, true)
		default:
			lastSuccessfullyProcessedUserID = data.UserID
			continue
		}
	}

	// mark as completed
	sm.Completed = true
	_, err = c.db.UpsertStreamMetadata(ctx, sm)

	if err == nil {
		log.Printf("streaming completed successfully")
	}

	// mark the streaming as finished
	<-c.StreamStartedChannel

	return err
}

func (s *Controller) processData(data *pb.Data, streamID string) error {
	if data.Value == 6 {
		return fmt.Errorf("mock error")
	}

	if data.Value == 7 {
		panic("oops panic")
	}

	err := s.db.UpsertData(context.Background(), data, streamID)
	return err
}

type InterruptionMessage struct {
	StreamID string `json:"streamID"`
}

func (c *Controller) ShutDownStreamingGracefully(ctx context.Context, sm db.StreamMetadata, lastUserIDStreamed string, thisServiceShuttingDown bool) error {

	if len(lastUserIDStreamed) > 0 {
		sm.LastUserIDStreamed = lastUserIDStreamed
	}

	_, err := c.db.UpsertStreamMetadata(ctx, sm)
	if err != nil {
		return err
	}

	b := InterruptionMessage{
		StreamID: sm.ID,
	}

	bytes, _ := json.Marshal(b)
	pub := amqp.PublishWithDefaults("client", "interrupted", bytes)
	err = c.broker.Publish(ctx, pub)
	if err != nil {
		log.Printf("error when publishing interruption message: %v", err)
	}

	// if we are gracefully interrupting the process due to client service going down, we want to decrease the wait group
	// but if we're interrupting because other dependency (server) returned an error,
	// we don't want to be decreasing the wait group because the graceful shutdown on this container is not invoked
	if thisServiceShuttingDown {
		defer func() {
			log.Printf("marking interruption as completed")
			c.WaitGroup.Done()
		}()
	}

	if err != nil {
		return fmt.Errorf("Error publishing evalutaion for all interruption message: %v", err.Error())
	}
	return nil
}

func (c *Controller) CarryOnInterruptedStreaming(ctx context.Context, msg InterruptionMessage) error {

	d, err := c.db.GetPointOfInterruption(ctx, msg.StreamID)
	if err != nil {
		log.Printf("error getting point of interruption: %v", err)
		return err
	}

	err = c.PerformStreaming(ctx, d.Value)

	if err != nil {
		log.Printf("error when carrying on streaming %v: %v", msg, err)
	}
	return err
}
