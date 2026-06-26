package orchestrator

import (
	"sync"
	"time"
)

// Event is a single bus event that carries all fields from the four SSE
// event shapes (node_status, request_status, task, audit). Unused fields
// are omitted from JSON serialization.
type Event struct {
	Type       string    `json:"type"`
	RequestID  string    `json:"request_id"`
	NodeID     string    `json:"node_id,omitempty"`
	Key        string    `json:"key,omitempty"`
	Status     string    `json:"status,omitempty"`
	Progress   int       `json:"progress_percent,omitempty"`
	StatusText string    `json:"status_text,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	Title      string    `json:"title,omitempty"`
	Actor      string    `json:"actor,omitempty"`
	Action     string    `json:"action,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	At         time.Time `json:"at"`
}

// Bus is an in-process publish/subscribe message bus keyed by request id.
// Publishers call Publish to fan out to every subscriber for that request;
// the SSE endpoint calls Subscribe to receive events as a channel.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]map[string]chan Event
	seq         int
}

func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[string]map[string]chan Event),
	}
}

// Subscribe registers a channel for a request id. It returns a read-only
// channel and a cleanup function. When ctx is done the subscriber is
// automatically removed. The channel is buffered (capacity 32) so a slow
// subscriber does not block publishers; overflowing messages are dropped.
func (b *Bus) Subscribe(requestID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)

	b.mu.Lock()
	b.seq++
	subID := b.subKey(b.seq)
	if b.subscribers[requestID] == nil {
		b.subscribers[requestID] = make(map[string]chan Event)
	}
	b.subscribers[requestID][subID] = ch
	b.mu.Unlock()

	cleanup := func() {
		b.mu.Lock()
		delete(b.subscribers[requestID], subID)
		if len(b.subscribers[requestID]) == 0 {
			delete(b.subscribers, requestID)
		}
		b.mu.Unlock()
	}

	return ch, cleanup
}

// Publish fans an event out to every subscriber for its request id.
// It never blocks: if a subscriber's buffer is full the event is dropped.
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	subs := b.subscribers[event.RequestID]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Bus) subKey(seq int) string {
	return "s" + time.Now().Format("150405") + "_" + itoa(seq)
}

// itoa is a small non-allocating int-to-string for the sub key.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
