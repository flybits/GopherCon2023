package config

// Global defines the global configuration values
var Global = struct {
	RabbitmqScheme   string
	RabbitmqUsername string
	RabbitmqPassword string
	RabbitmqAddress  string
	RabbitmqPort     uint16
	RabbitmqVhost    string
}{

	RabbitmqScheme:   "amqp",
	RabbitmqUsername: "default_user_4F1GftH2GNUMLxisw07",
	RabbitmqPassword: "_Ltm36EgSPXLseoIUwkLcRAYEQh7fGbj",
	RabbitmqAddress:  "definition.test-rabbitmq.svc",
	RabbitmqPort:     5672,
	RabbitmqVhost:    "/",
}
