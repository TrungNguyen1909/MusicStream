package queue

import "testing"

func TestNew(t *testing.T) {
	q := New()
	if q == nil {
		t.Error("New() = ", q)
	}
}
func TestSize(t *testing.T) {
	q := New()
	for i := 0; i < 10; i++ {
		q.Push(struct{}{})
	}
	if q.Size() != 10 {
		t.Error("q.Size() != 10")
	}
	if q.Empty() {
		t.Error("q.Empty() == true")
	}
}

func TestPushOne(t *testing.T) {
	q := New()
	q.Push(true)
	if q.Size() != 1 {
		t.Error("q.Size() != 1")
	}
	if q.Front().(bool) != true {
		t.Error("q.Front().(bool) != true")
	}
	if q.Pop().(bool) != true {
		t.Error("q.Pop().(bool) != true")
	}
	if !q.Empty() {
		t.Error("q.Empty() == false")
	}
}
func TestPushTwo(t *testing.T) {
	q := New()
	q.Push(true)
	q.Push(false)
	if q.Size() != 2 {
		t.Error("q.Size() != 2")
	}
	if q.Front().(bool) != true {
		t.Error("q.Front().(bool) != true")
	}
	if q.Pop().(bool) != true {
		t.Error("q.Pop().(bool) != true")
	}
	if q.Size() != 1 {
		t.Error("q.Size() != 1")
	}
	if q.Front().(bool) != false {
		t.Error("q.Front().(bool) != false")
	}
	if q.Pop().(bool) != false {
		t.Error("q.Pop().(bool) != false")
	}
}

func TestPushThreadSafe(t *testing.T) {
	q := New()
	done := make(chan int, 6)
	for i := 0; i < 6; i++ {
		go func(q *Queue) { q.Push(1); done <- 1 }(q)
	}
	for i := 0; i < 6; i++ {
		<-done
	}
	if q.Size() != 6 {
		t.Error("q.Size() != 6")
	}
}

func TestPushPopThreadSafe(t *testing.T) {
	q := New()
	done := make(chan int, 1000)
	for i := 0; i < 1000; i++ {
		go func(q *Queue) { q.Push(1); done <- 1 }(q)
	}

	for i := 0; i < 1000; i++ {
		go func(q *Queue) { q.Pop(); done <- 1 }(q)
	}
	for i := 0; i < 2000; i++ {
		<-done
	}
	if sz := q.Size(); sz != 0 {
		t.Errorf("q.Size() = %d != 0", sz)
	}
}

func TestValues(t *testing.T) {
	q := New()
	for i := 1; i < 6; i++ {
		q.Push(i * i)
	}
	expected := []interface{}{1, 4, 9, 16, 25}
	result := q.Values()
	if len(result) != len(expected) {
		t.Error("len(result) != len(expected)")
	}
	for i, v := range result {
		if expected[i] != v.(int) {
			t.Error("expected[i] != v")
		}
	}
}

func TestValuesEmpty(t *testing.T) {
	q := New()
	if len(q.Values()) != 0 {
		t.Error("len(q.Values()) != 0")
	}
}

func TestRemove(t *testing.T) {
	q := New()
	for i := 1; i < 6; i++ {
		q.Push(i * i)
	}
	expected := []interface{}{1, 9, 16, 25}
	q.Remove(func(v interface{}) bool {
		return (v.(int) % 2) == 0
	})
	result := q.Values()
	if len(result) != len(expected) {
		t.Error("len(result) != len(expected)")
	}
	for i, v := range result {
		if expected[i] != v.(int) {
			t.Error("expected[i] != v")
		}
	}
	if q.Remove(func(v interface{}) bool { return false }) != nil {
		t.Error("q.Remove(func(v interface{}) bool { return false }) != nil")
	}
}

func TestPushPopWaitThreadSafe(t *testing.T) {
	q := New()
	result := make(chan int, 1000)
	done := make(chan int, 2000)
	for i := 0; i < 1000; i++ {
		go func(q *Queue) { result <- q.Pop().(int) }(q)
	}
	for i := 0; i < 2000; i++ {
		go func(q *Queue, i int) { q.Push(i * i); done <- 1 }(q, i)
	}
	for i := 0; i < 2000; i++ {
		<-done
	}
	var a map[int]bool
	for i := 0; i < 1000; i++ {
		v := <-result
		if appeared, ok := a[v]; appeared || ok {
			t.Error("Duplicate elements")
		}
	}
	if q.Size() != 1000 {
		t.Error("q.Size() != 1000")
	}
}

func TestCallbacks(t *testing.T) {
	q := New()
	c := make(chan int, 3)
	q.PushCallback = func(i interface{}) {
		i.(chan int) <- 1
	}
	q.PopCallback = func(i interface{}) {
		i.(chan int) <- 2
	}
	q.Push(c)
	q.Front()
	q.Pop()
	if <-c != 1 || <-c != 2 || len(c) > 0 {
		t.Error("Callbacks failed to be executed")
	}

}
