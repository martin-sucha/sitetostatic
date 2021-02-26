package scraper

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueue(t *testing.T) {
	initialTasks := []*task{
		{
			key: "0",
		},
	}
	in := make(chan *task)
	done := make(chan *task)
	out := make(chan *task)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(in)
		defer close(done)
		defer close(out)
		queue(initialTasks, in, done, out)
	}()

	var receivedKeys []string
	var num int
	for t := range out {
		receivedKeys = append(receivedKeys, t.key)
		if num < 50 {
			num++
			in <- &task{
				key: strconv.Itoa(num),
			}
			in <- &task{
				key: strconv.Itoa(num),
			}
		}
		done <- t
	}

	expectedKeys := make([]string, 51)
	for i := 0; i <= 50; i++ {
		expectedKeys[i] = strconv.Itoa(i)
	}

	assert.Equal(t, expectedKeys, receivedKeys)
}

func TestLinkedQueue_PushRight(t *testing.T) {
	var lq linkedQueue
	assert.Equal(t, []string{}, keys(lq.toSlice()))
	lq.pushRight(&task{key: "A"})
	assert.Equal(t, []string{"A"}, keys(lq.toSlice()))
	lq.pushRight(&task{key: "B"})
	assert.Equal(t, []string{"A", "B"}, keys(lq.toSlice()))
	lq.pushRight(&task{key: "C"})
	assert.Equal(t, []string{"A", "B", "C"}, keys(lq.toSlice()))
}

func TestLinkedQueue_PushLeft(t *testing.T) {
	var lq linkedQueue
	assert.Equal(t, []string{}, keys(lq.toSlice()))
	lq.pushLeft(&task{key: "A"})
	assert.Equal(t, []string{"A"}, keys(lq.toSlice()))
	lq.pushLeft(&task{key: "B"})
	assert.Equal(t, []string{"B", "A"}, keys(lq.toSlice()))
	lq.pushLeft(&task{key: "C"})
	assert.Equal(t, []string{"C", "B", "A"}, keys(lq.toSlice()))
}

func TestLinkedQueue_PopLeft(t *testing.T) {
	var lq linkedQueue
	lq.pushRight(&task{key: "A"})
	lq.pushRight(&task{key: "B"})
	lq.pushRight(&task{key: "C"})
	if !assert.ObjectsAreEqual([]string{"A", "B", "C"}, keys(lq.toSlice())) {
		t.Skipf("prerequisite for test not satisfied, see TestLinkedQueue_PushRight")
	}
	assert.Equal(t, &task{key: "A"}, lq.popLeft())
	assert.Equal(t, &task{key: "B"}, lq.popLeft())
	assert.Equal(t, &task{key: "C"}, lq.popLeft())
	assert.Nil(t, lq.popLeft())
	assert.Nil(t, lq.popLeft())
}

func keys(tasks []*task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.key)
	}
	return out
}
