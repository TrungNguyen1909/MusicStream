/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020 Nguyễn Hoàng Trung(TrungNguyen1909)
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

//Queue is a thread-safe FIFO queue data structure
type Queue struct {
	queue        *list.List
	mux          sync.RWMutex
	enqueued     *sync.Cond
	PushCallback func(interface{})
	PopCallback  func(interface{})
}

//Push inserts a new element e with value v to the back of queue c
func (c *Queue) Push(v interface{}) {
	c.mux.Lock()
	c.queue.PushBack(v)
	if c.PushCallback != nil {
		c.PushCallback(v)
	}
	c.mux.Unlock()
	c.enqueued.Signal()
}

//Front fetch the front element of c
func (c *Queue) Front() interface{} {
	c.enqueued.L.Lock()
	defer c.enqueued.L.Unlock()
	for c.Size() <= 0 {
		c.enqueued.Wait()
	}
	c.mux.RLock()
	defer c.mux.RUnlock()
	ele := c.queue.Front()
	return ele.Value
}

//Pop removes the front element of l and returns it. If the queue is empty, Pop waits for a new element to be enqueued, removes it and returns the value
func (c *Queue) Pop() interface{} {
	c.enqueued.L.Lock()
	defer c.enqueued.L.Unlock()
	for c.Size() <= 0 {
		c.enqueued.Wait()
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	ele := c.queue.Front()
	c.queue.Remove(ele)
	if c.PopCallback != nil {
		c.PopCallback(ele.Value)
	}
	return ele.Value
}

//Size returns the number of elements in c
func (c *Queue) Size() int {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.queue.Len()
}

//Empty returns whether there's no elements in c
func (c *Queue) Empty() bool {
	return c.Size() == 0
}

//Remove removes and returns the first element that Predicate(element.Value) returns true
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

//Values returns all the elements of c in a slice
func (c *Queue) Values() (elements []interface{}) {
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

//New returns a new empty Queue
func New() *Queue {
	return &Queue{
		queue:    &list.List{},
		enqueued: sync.NewCond(&sync.Mutex{}),
	}
}
