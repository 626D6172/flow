package flow

import (
	"context"
	"sync"
)

type Node struct {
	ctx       context.Context
	wg        sync.WaitGroup
	completed bool

	EnableGoRoutineSend bool                                                         // If true, sends data to the next nodes in a goroutine.
	ErrorMethod         func(context.Context, error, ...interface{})                 // Error handling method, called if the Method function returns an error.
	Nodes               []*Node                                                      // Linked nodes that will be executed after the current node's method completes.
	Method              func(context.Context, ...interface{}) ([]interface{}, error) // Method function that processes input data and returns output data to the next nodes.
	Queue               chan []interface{}                                           // Channel for sending data to the workers.  Do not modify this field directly.
	ValidFunc           func(context.Context, ...interface{}) bool                   // Validation function that checks if the input data is valid for processing by the Method function.
	WorkersCount        int                                                          // Number of worker goroutines that will process the input data simultaneously.
}

// Start initializes the flow and starts the worker goroutines for your flow node.
func (f *Node) Start(ctx context.Context) {
	if f.ctx != nil {
		return // Flow is already started
	}

	f.ctx = ctx
	f.Queue = make(chan []interface{}, f.WorkersCount)
	for w := 0; w < f.WorkersCount; w++ {
		f.wg.Add(1)
		go f.worker()
	}

	for _, next := range f.Nodes {
		if next != nil {
			next.Start(ctx)
		}
	}
}

// Send sends data to the flow node's queue for processing.
func (f *Node) Send(inter ...interface{}) {
	select {
	case <-f.ctx.Done():
		return
	case f.Queue <- inter:
	}
}

// Link links the current flow node to one or more subsequent flow nodes.
// Each linked node will be executed after the current node's method completes.
// The linked nodes will receive the output of the current node's method.
func (f *Node) Link(flows ...*Node) {
	f.Nodes = append(f.Nodes, flows...)
}

func (f *Node) isValid(ctx context.Context, inter ...interface{}) bool {
	return f.ValidFunc == nil || f.ValidFunc(ctx, inter...)
}

// Wait blocks until all workers in the flow node and its linked nodes have completed their processing.
func (f *Node) Wait() {
	if f.completed {
		return // Flow is already completed
	}

	f.completed = true
	for _, next := range f.Nodes {
		next.Wait()
	}

	f.wg.Wait()
}

// worker is the main processing function for each worker in the flow node.
// It listens for input on the Queue channel, processes the input using the Method function,
// and sends the result to any linked flows or directors.
// If an error occurs during processing, it calls the ErrorMethod if defined.
func (f *Node) worker() {
	defer f.wg.Done()
	for {
		select {
		case <-f.ctx.Done():
			return

		case interfaces := <-f.Queue:
			methodCtx := f.ctx
			resp, err := f.Method(methodCtx, interfaces...)

			if err != nil {
				if f.ErrorMethod != nil {
					f.ErrorMethod(methodCtx, err, interfaces...)
					continue
				}
			}

			for _, next := range f.Nodes {
				for _, inter := range resp {
					if !next.isValid(f.ctx, inter) {
						continue
					}

					if next.EnableGoRoutineSend {
						go next.Send(inter)
					} else {
						next.Send(inter)
					}
				}
			}
		}
	}
}
