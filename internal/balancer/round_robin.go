package balancer

import "sync/atomic"

type RoundRobin struct {
	counter atomic.Uint64
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (rr *RoundRobin) Next(n int) int {
	if n <= 0 {
		panic("round robin requires at least one upstream")
	}

	return int((rr.counter.Add(1) - 1) % uint64(n))
}
