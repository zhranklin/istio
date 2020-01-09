package version

import (
	"sync"
	"time"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/pkg/log"
)

// Queue of work tickets processed using a rate-limiting loop
type Queue interface {
	// Push a ticket
	Push(Task)
	// Run the loop until a signal on the channel
	Run(<-chan struct{})
}

// Handler specifies a function to apply on an object for a given event type
type Handler func() error

type ErrHandler func(queue Queue, task Task, event model.Event)

type RetryPolicy struct {
	NeverRetry     bool
	RetryTimesLeft int
	RetryInterval  time.Duration
}

// Task object for the event watchers; processes until handler succeeds
type Task struct {
	handler     Handler
	retryPolicy RetryPolicy
}

// NewTask creates a task from a work item
func NewTask(handler Handler, retryPolicy RetryPolicy) Task {
	return Task{handler: handler, retryPolicy: retryPolicy}
}

func (task Task) tryRetry() (Task, bool) {
	retryPolicy := task.retryPolicy
	task.retryPolicy.RetryTimesLeft--
	if retryPolicy.NeverRetry || retryPolicy.RetryTimesLeft <= 0 {
		return task, false
	}
	return task, true
}

type queueImpl struct {
	queue   []Task
	cond    *sync.Cond
	closing bool
}

// NewQueue instantiates a queue with a processing function
func NewQueue() Queue {
	return &queueImpl{
		queue:   make([]Task, 0),
		closing: false,
		cond:    sync.NewCond(&sync.Mutex{}),
	}
}

func (q *queueImpl) Push(item Task) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if !q.closing {
		q.queue = append(q.queue, item)
	}
	q.cond.Signal()
}

func (q *queueImpl) Run(stop <-chan struct{}) {
	go func() {
		<-stop
		q.cond.L.Lock()
		q.closing = true
		q.cond.L.Unlock()
	}()

	for {
		q.cond.L.Lock()
		for !q.closing && len(q.queue) == 0 {
			q.cond.Wait()
		}

		if len(q.queue) == 0 {
			q.cond.L.Unlock()
			// We must be shutting down.
			return
		}

		var item Task
		item, q.queue = q.queue[0], q.queue[1:]
		q.cond.L.Unlock()
		if err := item.handler(); err != nil {
			if retryTask, ok := item.tryRetry(); ok {
				log.Infof("Work item handle failed (%v), retry after delay %v, retry time left %v", err, retryTask.retryPolicy.RetryInterval, retryTask.retryPolicy.RetryTimesLeft)
				time.AfterFunc(retryTask.retryPolicy.RetryInterval, func() {
					q.Push(retryTask)
				})
			}
		}

	}
}

// ChainHandler applies handlers in a sequence
type ChainHandler struct {
	funcs []Handler
}

// Apply is the handler function
func (ch *ChainHandler) Apply(obj interface{}, event model.Event) error {
	for _, f := range ch.funcs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

// Append a handler as the last handler in the chain
func (ch *ChainHandler) Append(h Handler) {
	ch.funcs = append(ch.funcs, h)
}
