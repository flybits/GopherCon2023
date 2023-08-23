package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/client/db"
	"github.com/flybits/gophercon2023/client/service"
	"github.com/flybits/gophercon2023/server/pb"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
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

	// mark the streaming as finished
	defer func() {
		<-c.StreamStartedChannel
	}()

	var sm db.StreamMetadata
	var err error

	log.Printf("starting streaming with id %v from offset: %v", streamID, offset)

	// get stream metadata if it exists
	sm, err = c.db.GetOngoingStreamMetadata(ctx, streamID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		// there is no in progress streaming for the stream id so create a new one
		sm = db.StreamMetadata{
			ID: streamID,
		}
	}

	// update metadata to track with pod is performing streaming
	podName := os.Getenv("CONFIG_POD_NAME")
	sm.PodName = podName
	sm.Offset = offset
	sm, err = c.db.UpsertStreamMetadata(ctx, sm)

	if err != nil {
		log.Printf("error inserting stream metadata %v", err)
		return err
	}

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
	if err != nil {
		log.Printf("error when receiving stream: %v", err)
		return err
	}

	log.Printf("marking stream metadata as complete")
	// mark as completed
	sm.Completed = true
	_, err = c.db.UpsertStreamMetadata(ctx, sm)

	if err == nil {
		log.Printf("streaming completed successfully")
	}
	return err
}

func (c *Controller) receiveStream(ctx context.Context, stream pb.Server_GetDataClient, sm db.StreamMetadata, lastSuccessfullyProcessedUserID string, errCh chan error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic: panic %v", r)
			c.receiveStream(ctx, stream, sm, lastSuccessfullyProcessedUserID, errCh)
		}
	}()

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			log.Printf("received end of stream")
			break
		}

		if err != nil {
			log.Printf("with error happening lastSuccessfullyProcessedUserID is %v", lastSuccessfullyProcessedUserID)
			log.Printf("Err returned from stream Recv: %v", err)
			err = c.ShutDownStreamingGracefully(ctx, sm, lastSuccessfullyProcessedUserID, false)
			if err != nil {
				log.Printf("error when shutting down streaming gracefully")
			}
			errCh <- fmt.Errorf("streaming interrupted due to error on receiving")
			return
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
			err = c.ShutDownStreamingGracefully(ctx, sm, data.UserID, true)
			if err != nil {
				log.Printf("error when shutting down streaming due to graceful shutdown: %v", err)
			}
			errCh <- fmt.Errorf("streaming is interrupted due to graceful shutdown")
			return
		default:
			lastSuccessfullyProcessedUserID = data.UserID
			continue
		}
	}
	log.Printf("finishing receiving streaming")
	errCh <- nil
	log.Printf("finished receiving streaming")
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

	sm, err := c.db.GetOngoingStreamMetadata(ctx, msg.StreamID)
	if err != nil {
		log.Printf("error carrying on interruption: %v", err)
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

		// this is in case there are no servers are available
		// sleep a little before requesting carrying on again

		time.Sleep(5 * time.Second)
		// send message again so that it is retried
		bytes, _ := json.Marshal(InterruptionMessage{
			StreamID: sm.ID,
		})

		pub := amqp.PublishWithDefaults("client", "interrupted", bytes)
		err = c.broker.Publish(ctx, pub)
		if err != nil {
			log.Printf("error when publishing interruption message: %v", err)
		}

	}
	return err
}
