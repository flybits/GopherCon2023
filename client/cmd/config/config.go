package config

import "time"

// Global defines the global configuration values
var Global = struct {
	RabbitmqScheme   string
	RabbitmqUsername string
	RabbitmqPassword string
	RabbitmqAddress  string
	RabbitmqPort     uint16
	RabbitmqVhost    string

	MongoAddress       string
	MongoUsername      string
	MongoPassword      string
	MongoDatabase      string
	MongoReplicaset    string
	MongoConnTimeout   time.Duration
	MongoAuthMechanism string
}{

	RabbitmqScheme:   "amqp",
	RabbitmqUsername: "",
	RabbitmqPassword: "",
	RabbitmqAddress:  "",
	RabbitmqPort:     5672,
	RabbitmqVhost:    "/",

	MongoAddress:       "mongodb4",
	MongoUsername:      "demo",
	MongoPassword:      "demo",
	MongoDatabase:      "client_db",
	MongoReplicaset:    "",
	MongoConnTimeout:   30,
	MongoAuthMechanism: "SCRAM-SHA-1",
}
