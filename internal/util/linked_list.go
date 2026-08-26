package util

// Element is an element in a LinkedList.
type Element[T any] struct {
	E    T
	Next *Element[T]
	prev *Element[T]
}

// LinkedList is the small double-ended queue used by the JSON reader and
// marshaler. Its zero value is ready for use.
type LinkedList[T any] struct {
	Head *Element[T]
	Tail *Element[T]
}

func (l *LinkedList[T]) Append(value T) {
	element := &Element[T]{E: value, prev: l.Tail}
	if l.Tail == nil {
		l.Head = element
	} else {
		l.Tail.Next = element
	}
	l.Tail = element
}

func (l *LinkedList[T]) IsEmpty() bool {
	return l.Head == nil
}

func (l *LinkedList[T]) Last() T {
	if l.Tail == nil {
		var zero T
		return zero
	}
	return l.Tail.E
}

func (l *LinkedList[T]) First() T {
	if l.Head == nil {
		var zero T
		return zero
	}
	return l.Head.E
}

func (l *LinkedList[T]) RemoveFirst() T {
	if l.Head == nil {
		var zero T
		return zero
	}
	element := l.Head
	l.Head = element.Next
	if l.Head == nil {
		l.Tail = nil
	} else {
		l.Head.prev = nil
	}
	return element.E
}

func (l *LinkedList[T]) RemoveLast() T {
	if l.Tail == nil {
		var zero T
		return zero
	}
	element := l.Tail
	l.Tail = element.prev
	if l.Tail == nil {
		l.Head = nil
	} else {
		l.Tail.Next = nil
	}
	return element.E
}
