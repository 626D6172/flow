package flow

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestFlow_StartAndSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	results := []interface{}{}

	flow := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			mu.Lock()
			defer mu.Unlock()
			results = append(results, inputs...)
			return inputs, nil
		},
		WorkersCount: 2,
	}

	flow.Start(ctx)
	for i := 1; i <= 6; i++ {
		flow.Send(i)
	}

	time.Sleep(100 * time.Millisecond) // Allow workers to process

	mu.Lock()
	defer mu.Unlock()
	if len(results) != 6 {
		t.Errorf("expected 6 results, got %d", len(results))
	}
}

func TestFlow_ErrorHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	errorsHandled := []error{}

	flow := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			for _, input := range inputs {
				if input.(int) == 3 {
					return nil, errors.New("test error")
				}
			}
			return inputs, nil
		},
		ErrorMethod: func(ctx context.Context, err error, inputs ...interface{}) {
			mu.Lock()
			defer mu.Unlock()
			errorsHandled = append(errorsHandled, err)
		},
		WorkersCount: 1,
	}

	flow.Start(ctx)
	for i := 1; i <= 6; i++ {
		flow.Send(i)
	}

	time.Sleep(100 * time.Millisecond) // Allow workers to process

	mu.Lock()
	defer mu.Unlock()
	if len(errorsHandled) != 1 {
		t.Errorf("expected 1 error handled, got %d", len(errorsHandled))
	}
}

func TestFlow_LinkFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	results := []interface{}{}

	childFlow := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			mu.Lock()
			defer mu.Unlock()
			results = append(results, inputs...)
			return inputs, nil
		},
		WorkersCount: 1,
	}

	root := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			return inputs, nil
		},
		WorkersCount: 1,
	}

	root.Link(childFlow)
	root.Start(ctx)
	childFlow.Start(ctx)

	for i := 1; i <= 3; i++ {
		root.Send(i)
	}

	time.Sleep(100 * time.Millisecond) // Allow workers to process

	mu.Lock()
	defer mu.Unlock()
	if len(results) != 3 {
		t.Errorf("expected 3 results in child flow, got %d", len(results))
	}
}

func TestDirector_LinkDirector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	results := []int{}

	root := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			return inputs, nil
		},
		WorkersCount: 1,
	}

	childFlow := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			mu.Lock()
			defer mu.Unlock()
			for _, input := range inputs {
				n := input.(int)
				results = append(results, n)
			}

			return inputs, nil
		},
		ValidFunc: func(ctx context.Context, inputs ...interface{}) bool {
			// Example condition: only process even numbers
			for _, input := range inputs {
				n := input.(int)
				if n%2 == 0 {
					return true
				}
			}
			return false
		},
		WorkersCount: 1,
	}

	root.Link(childFlow)

	root.Start(ctx)
	for i := 1; i <= 4; i++ {
		root.Send(i)
	}

	time.Sleep(100 * time.Millisecond) // Allow workers to process

	mu.Lock()
	defer mu.Unlock()
	expectedResults := []int{2, 4}
	if !reflect.DeepEqual(results, expectedResults) {
		t.Errorf("expected results to be %v, got %v", expectedResults, results)
	}
}

func TestDirector_ValidFunc(t *testing.T) {
	ctx := context.Background()

	node := &Node{
		ValidFunc: func(ctx context.Context, inputs ...interface{}) bool {
			for _, input := range inputs {
				if input == 42 {
					return true
				}
			}
			return false
		},
	}

	if !node.isValid(ctx, 42) {
		t.Errorf("expected isValid to return true for input 42")
	}

	if node.isValid(ctx, 1, 2, 3) {
		t.Errorf("expected isValid to return false for inputs 1, 2, 3")
	}
}

func TestFlow_Wait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	results := []interface{}{}

	childFlow := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			mu.Lock()
			defer mu.Unlock()
			results = append(results, inputs...)
			return inputs, nil
		},
		WorkersCount: 1,
	}

	root := &Node{
		Method: func(ctx context.Context, inputs ...interface{}) ([]interface{}, error) {
			return inputs, nil
		},
		WorkersCount: 1,
	}

	root.Link(childFlow)
	root.Start(ctx)
	childFlow.Start(ctx)

	for i := 1; i <= 3; i++ {
		root.Send(i)
	}

	time.Sleep(100 * time.Millisecond) // Allow workers to process

	mu.Lock()
	defer mu.Unlock()
	if len(results) != 3 {
		t.Errorf("expected 3 results in child flow, got %d", len(results))
	}

	cancel() // Cancel the context to stop the flow
	select {
	case <-waitWrapHelper(root):
	case <-time.NewTimer(500 * time.Millisecond).C:
		t.Fail()
	}
}

func waitWrapHelper(f *Node) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		f.Wait()
		out <- struct{}{}
	}()
	return out
}
