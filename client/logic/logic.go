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
	"strconv"
	"strings"
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

	err = c.receiveStream(ctx, stream, sm)
	if err != nil {
		log.Printf("error when receiving stream: %v", err)
		return err
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

func (c *Controller) receiveStream(ctx context.Context, stream pb.Server_GetDataClient, sm db.StreamMetadata) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic: panic %v", r)

			go func() {
				//log.Printf("will continue processing data from offset %d\n", offset)

				// continue with a new context so that it does not cancel prematurely
				err := c.receiveStream(context.Background(), stream, sm)
				if err != nil {
					log.Printf("error when performing streaming: %v", err)
				}
			}()
		}
	}()

	var lastSuccessfullyProcessedUserID string
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			log.Printf("received end of stream")
			break
		}

		if err != nil {
			log.Printf("Err var returned from stream Recv: %v", err)
			return c.ShutDownStreamingGracefully(ctx, sm, lastSuccessfullyProcessedUserID, false)
		}

		log.Printf("received data: %v", data)

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
			log.Printf("setting the last processed user id: %v", data.UserID)
			lastSuccessfullyProcessedUserID = data.UserID
			continue
		}
	}
	return nil
}

func (c *Controller) processData(data *pb.Data, streamID string) error {
	if data.Value == 6 {
		return fmt.Errorf("mock error")
	}

	if data.Value == 7 {
		panic("oops panic")
	}

	err := c.db.UpsertData(context.Background(), data, streamID)
	return err
}

type InterruptionMessage struct {
	StreamID string `json:"streamID"`
}

func (c *Controller) ShutDownStreamingGracefully(ctx context.Context, sm db.StreamMetadata, lastUserIDStreamed string, thisServiceShuttingDown bool) error {

	log.Printf("the lastUserIDStreamed is %v", lastUserIDStreamed)
	if len(lastUserIDStreamed) > 0 {
		sm.LastUserIDStreamed = lastUserIDStreamed
	}

	_, err := c.db.UpsertStreamMetadata(ctx, sm)
	if err != nil {
		return err
	}

	log.Printf("the stream metadata with id %v is updated", sm.ID)
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

	return nil
}

func (c *Controller) CarryOnInterruptedStreaming(ctx context.Context, msg InterruptionMessage) error {

	log.Printf("carrying on streaming %v", msg.StreamID)

	sm, err := c.db.GetStreamMetadata(ctx, msg.StreamID)
	if err != nil {
		log.Printf("error getting point of interruption: %v", err)
		return err
	}

	parts := strings.Split(sm.LastUserIDStreamed, "userID")
	if len(parts) != 2 {
		return fmt.Errorf("the lastUserID streamed was not able to provide us with the offset: %v", sm.LastUserIDStreamed)
	}
	i, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return fmt.Errorf("error converting %v to int: %v", parts[1], err)
	}

	err = c.PerformStreaming(ctx, int32(i)+1, msg.StreamID)

	if err != nil {
		log.Printf("error when carrying on streaming %v: %v", msg, err)

		// send message again so that it is retried
		bytes, _ := json.Marshal(sm)
		pub := amqp.PublishWithDefaults("client", "interrupted", bytes)
		err = c.broker.Publish(ctx, pub)
		if err != nil {
			log.Printf("error when publishing interruption message: %v", err)
		}

	}
	return err
}
