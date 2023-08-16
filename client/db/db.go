package db

import (
	"context"
	"errors"
	"fmt"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"github.com/flybits/gophercon2023/server/pb"
	"github.com/google/uuid"
	"go.elastic.co/apm/module/apmmongo/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"os"
	"strings"
	"time"
)

type Db struct {
	client *mongo.Client
}

func NewMongoDb() (*Db, error) {
	Db := &Db{}
	err := Db.Connect()
	if err != nil {
		return Db, err
	}
	return Db, err
}

// Connect connects to mongodb databases.
func (Db *Db) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), config.Global.MongoConnTimeout*time.Second)
	defer cancel()
	log.Printf("Will connect to mongodb at addresses: %s", config.Global.MongoAddress)

	// mongoDb address is stored as a list of addressed separated by comma
	ips := strings.SplitN(config.Global.MongoAddress, ",", -1)

	cr := options.Credential{
		AuthMechanism: config.Global.MongoAuthMechanism,
		Username:      config.Global.MongoUsername,
		Password:      config.Global.MongoPassword,
		PasswordSet:   false,
	}

	secondary := readpref.SecondaryPreferred()
	client, err := mongo.Connect(ctx, options.Client().SetHosts(ips).
		SetConnectTimeout(config.Global.MongoConnTimeout*time.Second).
		SetAuth(cr).
		SetReplicaSet(config.Global.MongoReplicaset).
		SetReadPreference(secondary).
		SetMonitor(apmmongo.CommandMonitor()))

	if err != nil {
		log.Printf("error connecting to mongodb: %s", err)
		return err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return err
	}
	log.Printf("connected to mongodb")
	Db.client = client

	return nil
}

func (Db *Db) Disconnect(ctx context.Context) error {
	if err := Db.client.Disconnect(ctx); err != nil {
		return err
	}

	return nil
}

type StreamMetadata struct {
	ID                 string `json:"id" bson:"_id"`
	Offset             int32  `json:"offset" bson:"offset"`
	PodName            string `json:"podName" bson:"podName"`
	Completed          bool   `json:"completed" bson:"completed"`
	LastUserIDStreamed string `json:"lastUserIDStreamed" bson:"lastUserIDStreamed"`
}

type Data struct {
	UserID   string `json:"userID" bson:"_id"`
	Value    int32  `json:"value" bson:"value"`
	StreamID string `json:"streamID" bson:"streamID"`
}

func (d *Db) UpsertStreamMetadata(ctx context.Context, sm StreamMetadata) (StreamMetadata, error) {

	podName := os.Getenv("CONFIG_POD_NAME")
	if len(sm.ID) < 1 {
		sm.ID = uuid.New().String()
	}

	sm.PodName = podName
	query := bson.M{"_id": bson.M{"$eq": sm.ID}}
	update := bson.M{"$set": sm}
	ops := options.Update().SetUpsert(true)
	_, err := d.client.Database("client").Collection("streams").UpdateOne(ctx, query, update, ops)
	return sm, err
}

func (d *Db) UpsertData(ctx context.Context, data *pb.Data, streamID string) error {
	da := Data{
		UserID:   data.UserID,
		Value:    data.Value,
		StreamID: streamID,
	}
	query := bson.M{"_id": bson.M{"$eq": da.UserID}}
	update := bson.M{"$set": da}
	ops := options.Update().SetUpsert(true)
	_, err := d.client.Database("client").Collection("data").UpdateOne(ctx, query, update, ops)
	return err
}

func (d *Db) GetPointOfInterruption(ctx context.Context, streamId string) (Data, error) {
	query := bson.M{"streamID": bson.M{"$eq": streamId}}
	ops := options.Find().SetSort(bson.M{"_id": -1}).SetLimit(1)
	cur, err := d.client.Database("client").Collection("data").Find(ctx, query, ops)
	defer cur.Close(ctx)
	if err != nil {
		return Data{}, err
	}
	da := []Data{}
	err = cur.All(ctx, da)

	if err != nil {
		log.Printf("error when getting cur.All %v", err)
		return Data{}, err
	}

	if len(da) < 1 {
		log.Printf("the list in empty")
		return Data{}, fmt.Errorf("the list is empty")
	}
	return da[0], nil
}

func (d *Db) GetOngoingStreamWithPodName(ctx context.Context, podName string) (StreamMetadata, error) {
	query := bson.M{"podName": bson.M{"$eq": podName}, "completed": bson.M{"$eq": false}}
	r := d.client.Database("client").Collection("data").FindOne(ctx, query)
	if errors.Is(r.Err(), mongo.ErrNoDocuments) {
		// there is no in progress streaming for the terminated pod
		return StreamMetadata{}, r.Err()
	}
	var sm StreamMetadata
	err := r.Decode(&sm)
	if err != nil {
		return StreamMetadata{}, err
	}
	return sm, nil

}
