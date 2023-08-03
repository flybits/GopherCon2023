package db

import (
	"context"
	"github.com/flybits/gophercon2023/client/cmd/config"
	"go.elastic.co/apm/module/apmmongo/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
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
