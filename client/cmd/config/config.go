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
	RabbitmqUsername: "",
	RabbitmqPassword: "",
	RabbitmqAddress:  "",
	RabbitmqPort:     5672,
	RabbitmqVhost:    "/",
}
