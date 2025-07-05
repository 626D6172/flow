package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bmarse/flow"
)

func root(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	return inters, nil
}

func appender(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	out := make([]interface{}, 0, len(inters))
	for _, inter := range inters {
		var str string
		var ok bool
		if str, ok = inter.(string); !ok {
			return nil, errors.New("expected string input")
		}

		out = append(out, str+" processed by appender node")
	}

	return out, nil
}

func adder(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	out := make([]interface{}, 0, len(inters))
	for _, inter := range inters {
		var num int
		var ok bool
		if num, ok = inter.(int); !ok {
			return nil, errors.New("expected integer input")
		}

		out = append(out, num+1) // Increment the number by 1
	}

	return out, nil
}

func printer(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
	for _, inter := range inters {
		fmt.Println(inter)
	}
	return inters, nil
}

func errorMethod(ctx context.Context, err error, inters ...interface{}) {
	// Handle error, e.g., log it or send it to a monitoring system.
	log.Default().Printf("Error in adder node: %v with inputs: %v\n", err, inters)
}

func main() {
	// Create the root flow node with a method that processes input strings
	// and returns them with a suffix.
	// The root node also has an error handling method that logs errors.
	// The error handling method is called if the root node method encounters an error.
	root := &flow.Node{
		Method:       root,
		WorkersCount: 1,
		ErrorMethod:  errorMethod,
	}

	// Create an adder node that will process integer inputs by incrementing them.
	// The adder node has a validation function that checks if the inputs are integers.
	// If the validation fails, the adder node will not process the input and will not call the adder method.
	adder := &flow.Node{
		Method:       adder,
		WorkersCount: 1,
		ErrorMethod:  errorMethod,
		ValidFunc: func(ctx context.Context, inters ...interface{}) bool {
			// Validate that all inputs are integers before passing to the adder node
			for _, inter := range inters {
				if _, ok := inter.(int); !ok {
					return false
				}
			}
			return true
		},
	}

	// Create an appender node that will process string inputs by appending a suffix.
	// The appender node has a validation function that checks if the inputs are strings.
	// If the validation fails, the appender node will not process the input and will not call the appender method.
	appender := &flow.Node{
		Method:       appender,
		WorkersCount: 1,
		ErrorMethod:  errorMethod,
		ValidFunc: func(ctx context.Context, inters ...interface{}) bool {
			// Validate that all inputs are string before passing to the appender node
			for _, inter := range inters {
				if _, ok := inter.(string); !ok {
					return false
				}
			}
			return true
		},
	}

	// Create a printer node that will print the output of the root node.
	// This node will receive the output of the root node and print it to the console.
	// The printer node does not have an error handling method, as it is not expected to fail.
	printer := &flow.Node{
		Method:       printer,
		WorkersCount: 1,
	}

	// Link the nodes together in the flow.
	// The root node will send its output to the adder and appender conditional director nodes.
	// The adder node will process integer inputs and send its output to the printer node.
	// The appender node will process string inputs and also send its output to the printer node
	root.Link(adder, appender)
	adder.Link(printer)
	appender.Link(printer)

	ctx, cancel := context.WithCancel(context.Background())
	root.Start(ctx)

	// Send some strings and ints to the root node for processing.
	root.Send("Hello, World!")
	root.Send(1337)
	root.Send("Another string to process")
	root.Send(123)
	root.Send(3.14) // This will be ignored since it is not an int or string

	time.Sleep(100 * time.Millisecond) // Allow time for processing

	cancel() // Cancel the context to stop the flow gracefully
	root.Wait()
}
