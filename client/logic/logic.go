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
	streamCh := make(chan bool)
	interruptionCh := make(chan bool)

	return Controller{
		ServerManager:        sm,
		broker:               b,
		db:                   d,
		StreamStartedChannel: streamCh,
		InterruptionChannel:  interruptionCh,
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

	sm, err := c.db.UpsertStreamMetadata(ctx, db.StreamMetadata{
		Offset: offset,
	})

	if err != nil {
		log.Printf("error inserting stream metadata")
	}

	stream, err := c.ServerManager.GetStreamFromServer(ctx, offset)

	if err != nil {
		log.Printf("error getting stream: %v", err)
		return err
	}

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Printf("Err var returned from stream Recv: %v", err)
			//return c.ShutDownEvaluationForAllWithStreamingGracefully(ctx, ru.ID, lastSuccessfullyProcessedUserID, lastSuccessfullyProcessedDeviceID, fireAndForget, origin, uniqueID, p, true)
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
			return c.ShutDownStreamingGracefully(ctx, sm, data.UserID)
		default:
			continue
		}
	}

	// mark as completed
	sm.Completed = true
	_, err = c.db.UpsertStreamMetadata(ctx, sm)
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

func (c *Controller) ShutDownStreamingGracefully(ctx context.Context, sm db.StreamMetadata, lastUserIDStreamed string, skipSendingInterruptionCompletedMessage bool) error {

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

	// if we are gracefully interrupting the process due to rules service going down, we want to send this message
	// so that main listens and controls when to close other things
	// but if we're interrupting because other dependency (ctxrepo) returned an error,
	// we don't want to be sending this because there would be no consumer to listen to it
	if !skipSendingInterruptionCompletedMessage {
		// send interruption completed message
		defer func() {
			logger.Info("sending interruption completed message")
			c.InterruptionCompleted <- true
			logger.Info("interruption completed message successfully sent")
		}()
	}

	if err != nil {
		return fmt.Errorf("Error publishing evalutaion for all interruption message: %v", err.Error())
	}
	return nil
}

func (c *Controller) CarryOnInterruptedStreaming(requestId string, msg InterruptionMessage) error {

	// the context has to be recreated from the background context in the goroutine, because the caller context with get cancelled when the request is over
	ctx := context.WithValue(context.Background(), "x-request-id", []string{requestId})
	logger := log.LoggerFromContext(ctx)
	logger = logger.With("requestId", requestId)
	ctx = log.ContextWithLogger(ctx, logger)

	p, err := c.RulesProcessForAllInfoRepository.GetRuleProcessForAllInfoByID(ctx, msg.ProcessForAllID)
	if err != nil {
		logger.Errorf("error getting process for all info by id: %v", err)
		return err
	}

	logger.Debugf("retrieved processForAllInfo: %v", p)
	r, err := c.RulesRepository.FindRule(ctx, p.RuleID.Hex(), p.TenantID)
	if err != nil {
		logger.Errorf("error retrieving rule from db: %v", err)
		return err
	}
	tenantID, _ := fbuuid.FromString(p.TenantID)

	err = c.EvaluateRuleForAllUsersInTenant(ctx, r, tenantID, msg.NewTenantScopedContext, msg.FireAndForget, msg.Origin, msg.UniqueID, p.LastRuleEvaluationID, &p)

	if err != nil {
		logger.Errorf("error when carrying on evaluation for all for msg %v: %v", msg, err)
	}
	return err
}
