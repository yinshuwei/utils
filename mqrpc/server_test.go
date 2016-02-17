package mqrpc

import (
	"errors"
	"flag"
	"fmt"
	"net/rpc"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

var (
	url    string
	queue  string
	server *Server
	once   sync.Once
)

type Args struct {
	A, B int
}

type Reply struct {
	C int
}

type Arith int

func (t *Arith) Add(args Args, reply *Reply) error {
	reply.C = args.A + args.B
	return nil
}

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Div(args Args, reply *Reply) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	reply.C = args.A / args.B
	return nil
}

func (t *Arith) String(args *Args, reply *string) error {
	*reply = fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	return nil
}

func (t *Arith) Scan(args string, reply *Reply) (err error) {
	_, err = fmt.Sscan(args, &reply.C)
	return
}

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

func init() {
	flag.StringVar(&url, "url", "amqp://guest:guest@localhost:5672/", "amqp url for testing")
	queue = "rpc.test"
}

func startServer() {
	server = NewServer()
	server.Register(new(Arith))
	server.RegisterName("net.rpc.Arith", new(Arith))
	server.RegisterName("server.Arith", new(Arith))
	go server.Serve(url, queue)
}

func dial() (*Client, error) {
	return Dial(url)
}

func TestRPC(t *testing.T) {
	once.Do(startServer)

	client, err := dial()
	if err != nil {
		t.Fatal("dialing", err)
	}
	defer client.Close()

	// Synchronous calls
	args := &Args{7, 8}
	reply := new(Reply)
	err = client.Call(queue, "Arith.Add", args, reply)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}

	// Nonexistent method
	args = &Args{7, 0}
	reply = new(Reply)
	err = client.Call(queue, "Arith.BadOperation", args, reply)
	// expect an error
	if err == nil {
		t.Error("BadOperation: expected error")
	} else if !strings.HasPrefix(err.Error(), "rpc: can't find method ") {
		t.Errorf("BadOperation: expected can't find method error; got %q", err)
	}

	// Unknown service
	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call(queue, "Arith.Unknown", args, reply)
	if err == nil {
		t.Error("expected error calling unknown service")
	} else if strings.Index(err.Error(), "method") < 0 {
		t.Error("expected error about method; got", err)
	}

	// Missing params
	reply = new(Reply)
	err = client.Call(queue, "Arith.Add", nil, reply)
	if err == nil {
		t.Error("expected error")
	} else if err.Error() != "mqrpc: request body missing params" {
		t.Error("expected request body missing params error; got", err)
	}

	// Out of order.
	args = &Args{7, 8}
	mulReply := new(Reply)
	mulCall := client.Go(queue, "Arith.Mul", args, mulReply, nil)
	addReply := new(Reply)
	addCall := client.Go(queue, "Arith.Add", args, addReply, nil)

	addCall = <-addCall.Done
	if addCall.Error != nil {
		t.Errorf("Add: expected no error but got string %q", addCall.Error.Error())
	}
	if addReply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", addReply.C, args.A+args.B)
	}

	mulCall = <-mulCall.Done
	if mulCall.Error != nil {
		t.Errorf("Mul: expected no error but got string %q", mulCall.Error.Error())
	}
	if mulReply.C != args.A*args.B {
		t.Errorf("Mul: expected %d got %d", mulReply.C, args.A*args.B)
	}

	// Error test
	args = &Args{7, 0}
	reply = new(Reply)
	err = client.Call(queue, "Arith.Div", args, reply)
	// expect an error: zero divide
	if err == nil {
		t.Error("Div: expected error")
	} else if err.Error() != "divide by zero" {
		t.Error("Div: expected divide by zero error; got", err)
	}

	// Non-struct argument
	const Val = 12345
	str := fmt.Sprint(Val)
	reply = new(Reply)
	err = client.Call(queue, "Arith.Scan", &str, reply)
	if err != nil {
		t.Errorf("Scan: expected no error but got string %q", err.Error())
	} else if reply.C != Val {
		t.Errorf("Scan: expected %d got %d", Val, reply.C)
	}

	// Non-struct reply
	args = &Args{27, 35}
	str = ""
	err = client.Call(queue, "Arith.String", args, &str)
	if err != nil {
		t.Errorf("String: expected no error but got string %q", err.Error())
	}
	expect := fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	if str != expect {
		t.Errorf("String: expected %s got %s", expect, str)
	}

	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call(queue, "Arith.Mul", args, reply)
	if err != nil {
		t.Errorf("Mul: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A*args.B {
		t.Errorf("Mul: expected %d got %d", reply.C, args.A*args.B)
	}

	// ServiceName contain "." character
	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call(queue, "net.rpc.Arith.Add", args, reply)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}
}

func TestClientClose(t *testing.T) {
	once.Do(startServer)

	client, err := dial()
	if err != nil {
		t.Fatalf("dialing: %v", err)
	}
	defer client.Close()

	args := Args{17, 8}
	var reply Reply
	err = client.Call(queue, "Arith.Mul", args, &reply)
	if err != nil {
		t.Fatal("arith error:", err)
	}
	t.Logf("Arith: %d*%d=%d\n", args.A, args.B, reply)
	if reply.C != args.A*args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A*args.B)
	}
}

func TestErrorAfterClientClose(t *testing.T) {
	once.Do(startServer)

	client, err := dial()
	if err != nil {
		t.Fatalf("dialing: %v", err)
	}
	err = client.Close()
	if err != nil {
		t.Fatal("close error:", err)
	}
	err = client.Call(queue, "Arith.Add", &Args{7, 9}, new(Reply))
	if err != ErrShutdown {
		t.Errorf("Forever: expected ErrShutdown got %v", err)
	}
}

func BenchmarkEndToEnd(b *testing.B) {
	b.StopTimer()
	once.Do(startServer)
	client, err := dial()
	if err != nil {
		b.Fatal("error dialing:", err)
	}
	defer client.Close()

	args := &Args{7, 8}
	procs := runtime.GOMAXPROCS(-1)
	N := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	b.StartTimer()

	for p := 0; p < procs; p++ {
		go func() {
			reply := new(Reply)
			for atomic.AddInt32(&N, -1) >= 0 {
				err := client.Call(queue, "Arith.Add", args, reply)
				if err != nil {
					b.Fatalf("rpc error: Add: expected no error but got string %q", err.Error())
				}
				if reply.C != args.A+args.B {
					b.Fatalf("rpc error: Add: expected %d got %d", reply.C, args.A+args.B)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkEndToEndAsync(b *testing.B) {
	const MaxConcurrentCalls = 100
	b.StopTimer()
	once.Do(startServer)
	client, err := dial()
	if err != nil {
		b.Fatal("error dialing:", err)
	}
	defer client.Close()

	args := &Args{7, 8}
	procs := 4 * runtime.GOMAXPROCS(-1)
	send := int32(b.N)
	recv := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	gate := make(chan bool, MaxConcurrentCalls)
	res := make(chan *rpc.Call, MaxConcurrentCalls)
	b.StartTimer()

	for p := 0; p < procs; p++ {
		go func() {
			for atomic.AddInt32(&send, -1) >= 0 {
				gate <- true
				reply := new(Reply)
				client.Go(queue, "Arith.Add", args, reply, res)
			}
		}()
		go func() {
			for call := range res {
				A := call.Args.(*Args).A
				B := call.Args.(*Args).B
				C := call.Reply.(*Reply).C
				if A+B != C {
					b.Fatalf("incorrect reply: Add: expected %d got %d", A+B, C)
				}
				<-gate
				if atomic.AddInt32(&recv, -1) == 0 {
					close(res)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
