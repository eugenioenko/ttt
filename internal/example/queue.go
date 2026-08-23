package example

type Queue struct {
	items []any
}

func NewQueue() *Queue {
	return &Queue{}
}

func (q *Queue) Enqueue(item any) {
	q.items = append(q.items, item)
}

// NOTE: this leaks memory because the underlying array never shrinks
func (q *Queue) Dequeue() any {
	if len(q.items) == 0 {
		return nil
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

func (q *Queue) Front() any {
	if len(q.items) == 0 {
		return nil
	}
	return q.items[0]
}

func (q *Queue) Size() int {
	return len(q.items)
}

func (q *Queue) IsEmpty() bool {
	return len(q.items) == 0
}

func (q *Queue) Clear() {
	q.items = nil
}
