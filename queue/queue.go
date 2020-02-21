/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020  Nguyễn Hoàng Trung
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package queue

import (
	"container/list"
	"sync"
)

//Queue is a wrapper around container/list. Queue is atomic
type Queue struct {
	queue    *list.List
	mux      sync.RWMutex
	enqueued *sync.Cond
}

//Enqueue atomically inserts a new element e with value v to the back of queue c
func (c *Queue) Enqueue(v interface{}) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.queue.PushBack(v)
	c.enqueued.Signal()
}

//Dequeue atomically removes the back element of l. If the queue is empty, Dequeue waits for a new element to be enqueued and then removes it
func (c *Queue) Dequeue() {
	if c.Size() <= 0 {
		c.enqueued.L.Lock()
		defer c.enqueued.L.Unlock()
		c.enqueued.Wait()
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	ele := c.queue.Front()
	c.queue.Remove(ele)
}

//Front atomically fetch the front element of c
func (c *Queue) Front() interface{} {
	if c.Size() <= 0 {
		c.enqueued.L.Lock()
		defer c.enqueued.L.Unlock()
		c.enqueued.Wait()
	}
	c.mux.RLock()
	defer c.mux.RUnlock()
	val := c.queue.Front().Value
	return val
}

//Pop atomically removes the back element of l and returns it. If the queue is empty, Pop waits for a new element to be enqueued, removes it and returns the value
func (c *Queue) Pop() interface{} {
	if c.Size() <= 0 {
		c.enqueued.L.Lock()
		defer c.enqueued.L.Unlock()
		c.enqueued.Wait()
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	ele := c.queue.Front()
	c.queue.Remove(ele)
	return ele.Value
}

//Size atomically returns the number of elements in c
func (c *Queue) Size() int {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.queue.Len()
}

//Empty atomically returns whether there's no elements in c
func (c *Queue) Empty() bool {
	return c.Size() == 0
}

//Remove atomically removes and returns the first element that Predicate(element.Value) returns true
func (c *Queue) Remove(Predicate func(v interface{}) bool) interface{} {
	c.mux.Lock()
	defer c.mux.Unlock()
	ele := c.queue.Front()
	for ele != nil {
		if Predicate(ele.Value) {
			c.queue.Remove(ele)
			return ele.Value
		}
		ele = ele.Next()
	}
	return nil
}

//GetElements atomically returns all the elements of c in a slice
func (c *Queue) GetElements() (elements []interface{}) {
	c.mux.RLock()
	defer c.mux.RUnlock()
	ele := c.queue.Front()
	elements = make([]interface{}, 0, c.queue.Len())
	for ele != nil {
		elements = append(elements, ele.Value)
		ele = ele.Next()
	}
	return
}

//NewQueue returns a new Queue
func NewQueue() *Queue {
	return &Queue{
		queue:    &list.List{},
		enqueued: sync.NewCond(&sync.Mutex{}),
	}
}
