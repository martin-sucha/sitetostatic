package scraper

import (
	"fmt"
	"net/url"
)

type task struct {
	downloadURL *url.URL
	key         string
	next        *task
}

// queue implements a task queue.
// It runs as long as there it at least one incomplete task.
// New tasks are posted to in and can be read out from out.
// A task is marked as complete by sending it to doneTask.
func queue(initialTasks []*task, in <-chan *task, doneTask <-chan *task, out chan<- *task) {
	addedKeys := make(map[string]struct{})
	var q linkedQueue
	for _, t := range initialTasks {
		if _, ok := addedKeys[t.key]; ok {
			// already added this key, skip it
			continue
		}
		q.pushRight(t)
	}
	incompleteTasks := len(initialTasks)
Loop:
	for incompleteTasks > 0 {
		var sendChan chan<- *task
		currentTask := q.popLeft()
		if currentTask != nil {
			sendChan = out
		}
		select {
		case t, ok := <-in:
			if currentTask != nil {
				// need to restore the task for next iteration.
				q.pushLeft(currentTask)
			}
			if !ok {
				in = nil
				continue Loop
			}
			if _, ok := addedKeys[t.key]; ok {
				// already added this key, skip it
				continue Loop
			}
			addedKeys[t.key] = struct{}{}
			q.pushRight(t)
			incompleteTasks++
		case sendChan <- currentTask:
			// successfully sent
		case _, ok := <-doneTask:
			if currentTask != nil {
				// need to restore the task for next iteration.
				q.pushLeft(currentTask)
			}
			if ok {
				incompleteTasks--
			}
		}
	}
}

type linkedQueue struct {
	head, tail *task
}

func (lq *linkedQueue) pushRight(t *task) {
	if lq.tail == nil {
		lq.head = t
		lq.tail = t
		return
	}
	lq.tail.next = t
	lq.tail = t
}

func (lq *linkedQueue) pushLeft(t *task) {
	t.next = lq.head
	lq.head = t
	if lq.tail == nil {
		lq.tail = lq.head
	}
}

func (lq *linkedQueue) popLeft() *task {
	if lq.head == nil {
		return nil
	}
	t := lq.head
	lq.head, t.next = t.next, nil
	if lq.head == nil {
		lq.tail = nil
	}
	return t
}

func (lq *linkedQueue) len() int {
	l := 0
	for cur := lq.head; cur != nil; cur = cur.next {
		l++
	}
	return l
}

func (lq *linkedQueue) String() string {
	return fmt.Sprintf("linkedQueue<len=%d>", lq.len())
}

func (lq *linkedQueue) toSlice() []*task {
	out := make([]*task, 0, lq.len())
	for cur := lq.head; cur != nil; cur = cur.next {
		out = append(out, cur)
	}
	return out
}
