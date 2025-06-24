## About
Flow is a package the allows for easy conditional branching Go routine workflows.

## Usage
There are two steps to get a work flow defined.  You must difine the worker methods and then define the node flow order, branches, and conditional logic.  Once you have this you can start running basic use cases.

### Defining your worker methods
To create workflows you need to have each of your workflow steps.
```golang
func step1(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
    // Do something

    // pass the output to the next step
    return inters, nil
}

func step2(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
    // ...
    return inters, nil
}

func step3(ctx context.Context, inters ...interface{}) ([]interface{}, error) {
    // ...
    return inters, nil
}
```

### Defining your flow
Each node in flow can have multiple nodes to run as the next step.  If you always what the output of the current worker to go into the next Node use `Link` without a `ValidFunc` set in the node.  If you want to have conditional logic (See `examples/conditional_printer/main.go`) use set a `ValidFunc` in the node.  The ValidFunc is a good way for verifying data quality and contents.  Once you know your flow you connect them using the appropriate validations.

```golang
func main() {
	step1 := &flow.Node{
		Method:       step1,
		WorkersCount: 1,
	}

    step2a := &flow.Node{
		Method:       step2a,
		WorkersCount: 1,
	}

    step2b := &flow.Node{
		Method:       step2a,
		WorkersCount: 1,
	}

    step3 := &flow.Node{
		Method:       step3,
		WorkersCount: 1,
	}

    step1.LinkFlow(step2a, step2b)
    step2a.LinkFlow(step3)
}
```

### Starting your flow
To start the workers you must initialize the root node, this will cascade and start all following nodes.  In this case step1 is our root node.
```golang
ctx, cancel := context.WithCancel(context.Background())
step1.Start(ctx)
```

### Sending data
To send data you simply have to identify the the flow node you wish to send data to and use the `Send()` method.  You can execute `Send()` from one worker to another and create cycles, but be warey of creating deadlocks. (See `examples/naive_spider/main.go` for a cycle example)

```golang
root.Send("Hello world")
```

### Waiting and cancelling
If you want a graceful exit use a `context.WithCancel` and cancel when appropriate.  After your `cancel()` use the `Wait()` method on your root node allowing your code to wrap up and propagate the cancel.
```golang
func main() {
    // Snip...

	ctx, cancel := context.WithCancel(context.Background())
	root.Start(ctx)

	// Send some strings to the root node for processing.
	root.Send("Hello, World!")

	cancel() // Cancel the context to stop the flow gracefully
	root.Wait()
}
```

### Error Handling
To handle errors apply an `ErrorMethod` that will be triggered when a worker of that specific node encounters a non-nil error.
```golang
	downloader := &flow.Node{
		Method:       downloader,
		WorkersCount: 2,
		ErrorMethod:  func(ctx context.Context, err error, inters ...interface{}) {
            log.Default().Printf("Error in adder node: %v with inputs: %v\n", err, inters)
        },
	}
```

### Cycles
Like in `examples/naive_spider/main.go` sometimes you have cycles in your node workflow.  If you do this it is very easy to create deadlocks.  To prevent this you can enable `EnableGoRoutineSend` on the node to use a go routine for sending to the next nodes instead of consecutively.
```golang
	downloader := &flow.Node{
		Method:       downloader,
		WorkersCount: 2,
		EnableGoRoutineSend: true,
	}
```