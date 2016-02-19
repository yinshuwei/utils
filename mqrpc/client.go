package mqrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

var ErrShutdown = rpc.ErrShutdown

type Client struct {
	url       string
	mu        sync.Mutex
	conn      *amqp.Connection
	clientMap map[string]*rpc.Client
	quit      bool
	quitOnce  sync.Once
}

func Dial(url string) (*Client, error) {
	if conn, err := amqp.Dial(url); err != nil {
		return nil, errors.New("Failed to connect to MQServer: " + err.Error())
	} else {
		return NewClientWithConn(conn, url), nil
	}
}

func NewClientWithConn(conn *amqp.Connection, url string) *Client {
	return &Client{
		url:       url,
		conn:      conn,
		clientMap: make(map[string]*rpc.Client),
	}
}

//reconn to MQ Server
func (client *Client) reconn() {
	if conn, err := amqp.Dial(client.url); err != nil {
		log.Println(err)
	} else {
		log.Println("reconn")
		client.conn = conn
	}
}

func (client *Client) Go(queue string, serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	if c, err := client.client(queue); err != nil {
		if done == nil {
			done = make(chan *rpc.Call, 1)
		} else {
			if cap(done) == 0 {
				panic("rpc: done channel is unbuffered")
			}
		}
		call := &rpc.Call{
			ServiceMethod: serviceMethod,
			Args:          args,
			Reply:         reply,
			Error:         err,
			Done:          done,
		}
		done <- call
		return call
	} else {
		return c.Go(serviceMethod, args, reply, done)
	}
}

func (client *Client) Close() error {
	var err error
	client.quitOnce.Do(func() {
		client.mu.Lock()
		defer client.mu.Unlock()
		client.quit = true
		err = client.conn.Close()
		client.clientMap = nil
	})
	return err
}

// Call
func (client *Client) Call(queue string, serviceMethod string, args interface{}, reply interface{}) error {
	if c, err := client.client(queue); err != nil {
		return err
	} else {
		return c.Call(serviceMethod, args, reply)
	}
}

func (client *Client) client(queue string) (*rpc.Client, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.quit {
		return nil, ErrShutdown
	}
	c, ok := client.clientMap[queue]
	if ok {
		return c, nil
	}
	ch, err := client.conn.Channel()
	if err != nil {
		client.reconn()
		return nil, errors.New("Failed to open a channel")
	}
	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when usused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, errors.New("Failed to declare a queue")
	}
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // autoAck
		false,  // exclusive
		false,  // noLocal
		false,  // noWait
		nil,    // arguments
	)
	if err != nil {
		return nil, errors.New("Failed to register a consumer")
	}
	codec := &clientCodec{
		queue:   queue,
		replyTo: q.Name,
		ch:      ch,
		msgs:    msgs,
		pending: make(map[uint64]string),
	}
	c = rpc.NewClientWithCodec(codec)
	client.clientMap[queue] = c
	return c, nil
}

type clientCodec struct {
	sync.Mutex
	queue   string
	replyTo string
	req     clientRequest
	resp    clientResponse
	ch      *amqp.Channel
	msgs    <-chan amqp.Delivery
	pending map[uint64]string
}

type clientRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type clientResponse struct {
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

func (r *clientResponse) reset() {
	r.Result = nil
	r.Error = nil
}

func (c *clientCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	c.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.Unlock()
	c.req.Method = r.ServiceMethod
	c.req.Params = body
	if b, err := json.Marshal(&c.req); err != nil {
		return err
	} else {
		return c.ch.Publish(
			"",      // exchange
			c.queue, // routing key
			false,   // mandatory
			false,   // immediate
			amqp.Publishing{
				ContentType:   "application/json",
				CorrelationId: strconv.FormatUint(r.Seq, 10),
				ReplyTo:       c.replyTo,
				Body:          b,
			})
	}
}

func (c *clientCodec) ReadResponseHeader(r *rpc.Response) error {
	msg := <-c.msgs
	c.resp.reset()
	if err := json.Unmarshal(msg.Body, &c.resp); err != nil {
		return err
	}
	seq, err := strconv.ParseUint(msg.CorrelationId, 0, 64)
	if err != nil {
		return err
	}
	c.Lock()
	r.Seq = seq
	r.ServiceMethod = c.pending[seq]
	delete(c.pending, seq)
	c.Unlock()

	r.Error = ""
	if c.resp.Error != nil || c.resp.Result == nil {
		x, ok := c.resp.Error.(string)
		if !ok {
			return fmt.Errorf("invalid error %v", c.resp.Error)
		}
		if x == "" {
			x = "unspecified error"
		}
		r.Error = x
	}
	return nil
}

func (c *clientCodec) ReadResponseBody(body interface{}) error {
	if body == nil {
		return nil
	}
	return json.Unmarshal(*c.resp.Result, body)
}

func (c *clientCodec) Close() error {
	return c.ch.Close()
}

// Json Call
func (client *Client) JsonCall(queue string, serviceMethod string, args *[]byte, reply *[]byte) error {
	if c, err := client.jsonClient(queue); err != nil {
		return err
	} else {
		// return c.Call(serviceMethod, args, reply)
		timeout := time.NewTimer(time.Second * 3)
		select {
		case call := <-c.Go(serviceMethod, args, reply, make(chan *rpc.Call, 1)).Done:
			return call.Error
		case <-timeout.C: //3s timeout
			return errors.New("timeout")
		}
	}
}
func (client *Client) jsonClient(queue string) (*rpc.Client, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.quit {
		return nil, ErrShutdown
	}
	c, ok := client.clientMap[queue]
	if ok {
		return c, nil
	}
	ch, err := client.conn.Channel()
	if err != nil {
		client.reconn()
		return nil, errors.New("Failed to open a channel")
	}
	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when usused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, errors.New("Failed to declare a queue")
	}
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // autoAck
		false,  // exclusive
		false,  // noLocal
		false,  // noWait
		nil,    // arguments
	)
	if err != nil {
		return nil, errors.New("Failed to register a consumer")
	}
	codec := &jsonClientCodec{
		queue:     queue,
		replyTo:   q.Name,
		ch:        ch,
		msgs:      msgs,
		pending:   make(map[uint64]string),
		clientMap: client.clientMap,
	}
	c = rpc.NewClientWithCodec(codec)
	client.clientMap[queue] = c
	return c, nil
}

type jsonClientCodec struct {
	sync.Mutex
	queue     string
	replyTo   string
	req       jsonClientRequest
	resp      jsonClientResponse
	ch        *amqp.Channel
	msgs      <-chan amqp.Delivery
	pending   map[uint64]string
	clientMap map[string]*rpc.Client
}

type jsonClientRequest struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
}

type jsonClientResponse struct {
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

func (r *jsonClientResponse) reset() {
	r.Result = nil
	r.Error = nil
}

func (c *jsonClientCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	c.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.Unlock()
	c.req.Method = r.ServiceMethod
	params, _ := body.(*[]byte)
	paramsRawMessage := json.RawMessage(*params)
	c.req.Params = &paramsRawMessage

	if b, err := json.Marshal(&c.req); err != nil {
		return err
	} else {
		return c.ch.Publish(
			"",      // exchange
			c.queue, // routing key
			false,   // mandatory
			false,   // immediate
			amqp.Publishing{
				ContentType:   "application/json",
				CorrelationId: strconv.FormatUint(r.Seq, 10),
				ReplyTo:       c.replyTo,
				Body:          b,
			})
	}
}

func (c *jsonClientCodec) ReadResponseHeader(r *rpc.Response) error {
	timeout := time.NewTimer(time.Second * 3)
	select {
	case msg := <-c.msgs:
		c.resp.reset()
		if err := json.Unmarshal(msg.Body, &c.resp); err != nil {
			return err
		}
		seq, err := strconv.ParseUint(msg.CorrelationId, 0, 64)
		if err != nil {
			return err
		}
		c.Lock()
		r.Seq = seq
		r.ServiceMethod = c.pending[seq]
		delete(c.pending, seq)
		c.Unlock()

		r.Error = ""
		if c.resp.Error != nil || c.resp.Result == nil {
			x, ok := c.resp.Error.(string)
			if !ok {
				return fmt.Errorf("invalid error %v", c.resp.Error)
			}
			if x == "" {
				x = "unspecified error"
			}
			r.Error = x
		}
	case <-timeout.C: //clear queue when 1 hour no message
		c.Close()
		return errors.New("timeout")
	}
	return nil
}

func (c *jsonClientCodec) ReadResponseBody(body interface{}) error {
	if body != nil {
		bodyReal := body.(*[]byte)
		*bodyReal = *c.resp.Result
	}
	return nil
}

func (c *jsonClientCodec) Close() error {
	delete(c.clientMap, c.queue)
	return c.ch.Close()
}
