package queue

import (
	"container/list"
	"sync"
)

type Queue struct {
	queue           *list.List
	mux             sync.RWMutex
	enqueued        *sync.Cond
	EnqueueCallback func(interface{})
	DequeueCallback func()
}

func (c *Queue) Enqueue(value interface{}) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.queue.PushBack(value)
	c.enqueued.Signal()
	if c.EnqueueCallback != nil {
		go c.EnqueueCallback(value)
	}
}

func (c *Queue) Dequeue() {
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.Size() <= 0 {
		c.enqueued.L.Lock()
		defer c.enqueued.L.Unlock()
		c.enqueued.Wait()
	}
	ele := c.queue.Front()
	c.queue.Remove(ele)
	if c.DequeueCallback != nil {
		go c.DequeueCallback()
	}
}

func (c *Queue) Front() interface{} {
	if c.Size() <= 0 {
		c.enqueued.L.Lock()
		defer c.enqueued.L.Unlock()
		c.enqueued.Wait()
		c.mux.RLock()
		defer c.mux.RUnlock()
	}
	val := c.queue.Front().Value
	return val
}
func (c *Queue) Pop() interface{} {
	if c.Size() <= 0 {
		c.enqueued.L.Lock()
		defer c.enqueued.L.Unlock()
		c.enqueued.Wait()
		c.mux.Lock()
		defer c.mux.Unlock()
	}
	ele := c.queue.Front()
	c.queue.Remove(ele)
	if c.DequeueCallback != nil {
		go c.DequeueCallback()
	}
	return ele.Value
}
func (c *Queue) Size() int {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.queue.Len()
}

func (c *Queue) Empty() bool {
	return c.Size() == 0
}

func NewQueue() *Queue {
	return &Queue{
		queue:    &list.List{},
		enqueued: sync.NewCond(&sync.Mutex{}),
	}
}
