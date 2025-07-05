package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bmarse/flow"
)

// root is the method for the root flow node.
func root(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	out := make([]interface{}, 0, len(inters))
	for _, inter := range inters {
		var str string
		var ok bool
		if str, ok = inter.(string); !ok {
			return nil, errors.New("expected string input")
		}

		out = append(out, str+" processed by root node")
	}

	return out, nil
}

// printer is a method for a flow node that prints its inputs to the console.
// This is the second node in the flow, which receives the output of the root node.
func printer(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	for _, inter := range inters {
		fmt.Println(inter)
	}
	return inters, nil
}

func main() {
	// Create the root flow node with a method that processes input strings
	// and returns them with a suffix.
	// The root node also has an error handling method that logs errors.
	// The error handling method is called if the root node method encounters an error.
	root := &flow.Node{
		Method:       root,
		WorkersCount: 1,
		ErrorMethod: func(ctx context.Context, err error, inters ...interface{}) {
			// Handle error, e.g., log it or send it to a monitoring system.
			log.Default().Printf("Error in root node: %v with inputs: %v\n", err, inters)
		},
	}

	// Create a printer node that will print the output of the root node.
	// This node will receive the output of the root node and print it to the console.
	// The printer node does not have an error handling method, as it is not expected to fail.
	printer := &flow.Node{
		Method:       printer,
		WorkersCount: 1,
	}

	root.Link(printer)

	ctx, cancel := context.WithCancel(context.Background())
	root.Start(ctx)

	// Send some strings to the root node for processing.
	root.Send("Hello, World!")
	root.Send("Another string to process")
	// This should trigger an error in the root node.
	root.Send(123)

	time.Sleep(100 * time.Millisecond) // Allow time for processing

	cancel() // Cancel the context to stop the flow gracefully
	root.Wait()
}
