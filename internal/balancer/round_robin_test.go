package balancer

import (
	"sync"
	"testing"
)

func TestRoundRobin(t *testing.T) {
	rr := NewRoundRobin()

	expected := []int{0, 1, 2, 0, 1, 2}

	for _, want := range expected {
		got := rr.Next(3)

		if got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	}
}

func TestRoundRobinConcurrent(t *testing.T) {
	rr := NewRoundRobin()

	const requests = 1000
	const upstreams = 3

	results := make(chan int, requests)

	var wg sync.WaitGroup

	for i := 0; i < requests; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			results <- rr.Next(upstreams)
		}()
	}

	wg.Wait()
	close(results)

	counts := make([]int, upstreams)

	for index := range results {
		if index < 0 || index >= upstreams {
			t.Fatalf("invalid upstream index: %d", index)
		}
		counts[index]++
	}

	for _, count := range counts {
		if count < 333 || count > 334 {
			t.Errorf("unexpected distribution: %v", counts)
		}
	}
}
