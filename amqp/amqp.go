package amqp

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
	"sync"
	"time"
)

// connection is the interface for method used from amqp.Connection
type connection interface {
	Close() error
	IsClosed() bool
	Channel() (*amqp.Channel, error)
}

// channel is the interface for method used from amqp.Channel
type channel interface {
	Close() error
	Cancel(consumer string, noWait bool) error
	Qos(prefetchCount, prefetchSize int, global bool) error
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	ExchangeDelete(name string, ifUnused, noWait bool) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error)
	QueueUnbind(name, key, exchange string, args amqp.Table) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
}

// amqpDialer is an implementation of dialer for amqp package
type amqpDialer struct{}

// Override for testing
var osHostname = os.Hostname

func getHostname() (string, error) {
	host, err := osHostname()
	if err != nil {
		return "", err
	}
	return host, nil
}

func (d *amqpDialer) Dial(url string) (connection, error) {
	uri, err := amqp.ParseURI(url)
	if err != nil {
		return nil, err
	}

	pod, err := getHostname()
	if err != nil {
		if uri.Scheme == "amqps" {
			return amqp.DialTLS(url, &tls.Config{InsecureSkipVerify: true})
		}
		return amqp.Dial(url)
	}
	return amqp.DialConfig(url, amqp.Config{
		Heartbeat: 10 * time.Second,
		Properties: map[string]interface{}{
			"product": pod,
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})
}

// dialer an interface for wrapping amqp.Dial
type dialer interface {
	Dial(url string) (connection, error)
}

type Broker struct {
	d    dialer
	conn connection
	ch   channel
	wg   *sync.WaitGroup

	exchanges []Exchange
	queues    []Queue

	uriScheme     string
	host          string
	port          uint16
	username      string
	password      string
	vhost         string
	prefetchCount int
	prefetchSize  int
}

// Consumer is an AMQP consumer
type Consumer struct {
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Handler   HandlerFunc
}

// HandlerFunc is a function type for handling incoming messages
type HandlerFunc func(context.Context, Delivery) error

// Delivery is same as amqp.Delivery, so consumers of this package do not need to import amqp package too
type Delivery amqp.Delivery

// Ack is same as amqp.Ack
func (d Delivery) Ack(multiple bool) error {
	return amqp.Delivery(d).Ack(multiple)
}

// Binding is an AMQP binding between a queue and an exchange
type Binding struct {
	Key      string
	Exchange string
	NoWait   bool
}

// Queue is an AMQP queue
type Queue struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Unbind     []Binding
	Bindings   []Binding
	Consumers  []Consumer
	Args       amqp.Table
}

// Exchange is an AMQP exchange
type Exchange struct {
	Name       string
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
}

// BrokerOption sets optional parameters for broker
type BrokerOption func(*Broker)

// URIScheme is the option for setting the URI schema default amqp but can be changed to amqps
func URIScheme(uriScheme string) BrokerOption {
	return func(b *Broker) {
		b.uriScheme = uriScheme
	}
}

// Address is the option for setting host and port
func Address(host string, port uint16) BrokerOption {
	return func(b *Broker) {
		b.host = host
		b.port = port
	}
}

// Credentials is the option for setting username and password
func Credentials(username, password string) BrokerOption {
	return func(b *Broker) {
		b.username = username
		b.password = password
	}
}

// Vhost is the option for setting virtual host
func Vhost(vhost string) BrokerOption {
	return func(b *Broker) {
		b.vhost = vhost
	}
}

// Prefetch is the option for setting prefetch count and size
func Prefetch(count, size int) BrokerOption {
	return func(b *Broker) {
		b.prefetchCount = count
		b.prefetchSize = size
	}
}

// SetupBroker creates a new instance of broker
func (b *Broker) SetupBroker(exchanges []Exchange, queues []Queue, opts ...BrokerOption) error {
	b.d = &amqpDialer{}
	b.exchanges = exchanges
	b.queues = queues
	b.wg = &sync.WaitGroup{}

	for _, opt := range opts {
		opt(b)
	}

	if b.uriScheme == "" {
		b.uriScheme = "amqp"
	}

	if b.host == "" {
		b.host = "localhost"
	}

	if b.port == 0 {
		b.port = 5672
	}

	if b.username == "" {
		b.username = "guest"
	}

	if b.password == "" {
		b.password = "guest"
	}

	if b.vhost == "" {
		b.vhost = "/"
	}

	if b.prefetchCount == 0 {
		b.prefetchCount = 100
	}

	if b.prefetchSize == 0 {
		b.prefetchSize = 0
	}

	err := b.connect()
	if err != nil {
		return err
	}

	err = b.setup()
	if err != nil {
		return err
	}

	return nil
}

func (b *Broker) connect() error {
	var err error

	//if we reconnect after channel exception, just re-use the existing connection
	if b.conn == nil || b.conn == (*amqp.Connection)(nil) {
		// Create a connection to AMQP server
		url := fmt.Sprintf("%s://%s:%s@%s:%d/%s", b.uriScheme, b.username, b.password, b.host, b.port, b.vhost)
		b.conn, err = b.d.Dial(url)
		if err != nil {
			return err
		}

	}

	if b.conn != (*amqp.Connection)(nil) && b.conn.IsClosed() {
		// Create a connection to AMQP server
		url := fmt.Sprintf("%s://%s:%s@%s:%d/%s", b.uriScheme, b.username, b.password, b.host, b.port, b.vhost)
		b.conn, err = b.d.Dial(url)
		if err != nil {
			return err
		}
	}

	// Get an AMQP channel
	b.ch, err = b.conn.Channel()
	if err != nil {
		return err
	}

	// Reconnect if the connection is lost
	errc := make(chan *amqp.Error)
	go b.reconnect(b.ch.NotifyClose(errc))

	return nil
}

func (b *Broker) reconnect(errc chan *amqp.Error) {
	for _ = range errc {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			if err := b.connect(); err == nil {
				if err := b.setup(); err == nil {
					ticker.Stop()
				}
			}
		}
	}
}

func (b *Broker) setup() error {
	// Sets quality of service for all existing and future consumers on this channel
	err := b.ch.Qos(b.prefetchCount, b.prefetchSize, false)
	if err != nil {
		return err
	}

	// Declare AMQP exchanges
	for _, e := range b.exchanges {
		err := b.ch.ExchangeDeclare(e.Name, e.Kind, e.Durable, e.AutoDelete, e.Internal, e.NoWait, nil)
		if err != nil {
			return err
		}
	}

	// Declare AMQP queues
	for _, q := range b.queues {
		if q.AutoDelete {
			if q.Args == nil {
				q.Args = map[string]interface{}{
					"x-expires": 60 * 1000, //1m
				}
			} else {
				q.Args["x-expires"] = 60 * 1000
			}
		}
		amqpQueue, err := b.ch.QueueDeclare(q.Name, q.Durable, q.AutoDelete, q.Exclusive, q.NoWait, q.Args)
		if err != nil {
			return err
		}

		// Unbind this queue from existing exchanges if necessary
		for _, qb := range q.Unbind {
			err := b.ch.QueueUnbind(amqpQueue.Name, qb.Key, qb.Exchange, nil)
			if err != nil {
				return err
			}
		}

		// Bind exchanges to the queue
		for _, qb := range q.Bindings {
			err := b.ch.QueueBind(amqpQueue.Name, qb.Key, qb.Exchange, qb.NoWait, nil)
			if err != nil {
				return err
			}
		}

		// Register AMQP consumers
		for _, c := range q.Consumers {
			delvc, err := b.ch.Consume(amqpQueue.Name, c.Consumer, c.AutoAck, c.Exclusive, c.NoLocal, c.NoWait, nil)
			if err != nil {
				return err
			}

			b.wg.Add(1)
			go b.consume(delvc, c.Handler)
		}
	}

	return nil
}

// SpanContext and RequestID should exist on headers, otherwise they will be created
func (b *Broker) consume(delvc <-chan amqp.Delivery, handler HandlerFunc) {
	defer b.wg.Done()
	for d := range delvc {
		// This function lets us use defer statements for each delivery
		func(d amqp.Delivery) {
			// Create a context
			ctx := context.Background()

			// Consume
			_ = handler(ctx, Delivery(d))

		}(d)
	}
}

func (b *Broker) ShutDownConsumersForQueues(ctx context.Context, queueNames []string) []error {

	var errors []error
	for _, q := range b.queues {
		for _, qn := range queueNames {
			if qn == q.Name {
				for _, c := range q.Consumers {
					if err := b.ch.Cancel(c.Consumer, false); err != nil {
						if errors == nil {
							errors = []error{}
						}
						errors = append(errors, err)
					}
				}
			}
		}
	}

	return errors
}

// ShutDown gracefully shuts down message processing
func (b *Broker) ShutDown(ctx context.Context) error {
	done := make(chan int)

	for _, q := range b.queues {
		for _, c := range q.Consumers {
			if err := b.ch.Cancel(c.Consumer, false); err != nil {
				// log error but continue
			}
		}
	}

	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// log completed
	case <-ctx.Done():
		// log context expired, closing connection and channel
	}

	if err := b.Close(); err != nil {
		// log error closing connection
		return err
	}

	return nil
}

// Close closes the AMQP channel and connection
func (b *Broker) Close() error {
	if err := b.ch.Close(); err != nil {
		return err
	}

	if err := b.conn.Close(); err != nil {
		return err
	}

	return nil
}

// Publish is an AMQP publish
type Publish struct {
	Exchange   string
	Key        string
	Mandatory  bool
	Immediate  bool
	Publishing Publishing
}

// Publishing is same as amqp.Publishing, so consumers of this package do not need to import amqp package too
type Publishing amqp.Publishing

// PublishWithDefaults creates a Publish with default values
func PublishWithDefaults(exchange, key string, body []byte) Publish {
	return Publish{
		Exchange:  exchange,
		Key:       key,
		Mandatory: false,
		Immediate: false,
		Publishing: Publishing{
			DeliveryMode: amqp.Transient,
			Headers:      amqp.Table{},
			ContentType:  "text/json",
			Body:         body,
		},
	}
}

// Publish sends an AMQP Publishing to an exchange
// SpanContext and RequestID should exist on context, otherwise they will be created
func (b *Broker) Publish(ctx context.Context, p Publish) error {

	err := b.ch.Publish(p.Exchange, p.Key, p.Mandatory, p.Immediate, amqp.Publishing(p.Publishing))

	if err != nil {
		return fmt.Errorf("could not publish message. %s", err)
	}

	return nil
}

// ExchangeWithDefaults creates an Exchange with default values
func ExchangeWithDefaults(name, kind string) Exchange {
	if kind == "" {
		kind = "topic"
	}

	return Exchange{
		Name:       name,
		Kind:       kind,
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		NoWait:     false,
	}
}

// BindingWithDefaults creates a Binding with default values
func BindingWithDefaults(key, exchange string) Binding {
	return Binding{
		Key:      key,
		Exchange: exchange,
		NoWait:   false,
	}
}

// ConsumerWithDefaults creates a Consumer with default values
func ConsumerWithDefaults(autoAck bool, handler HandlerFunc) Consumer {
	consumer, _ := uuid.NewUUID()
	return Consumer{
		Consumer:  consumer.String(),
		AutoAck:   autoAck,
		Exclusive: false,
		NoLocal:   false,
		NoWait:    false,
		Handler:   handler,
	}
}
