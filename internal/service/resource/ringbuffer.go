package resource

// RingBuffer is a generic fixed-size circular buffer
// Zero allocations on Push, predictable memory usage
type RingBuffer[T any] struct {
	buffer []T
	head   int // Next write position
	count  int // Valid entries (0 to cap)
}

// NewRingBuffer creates a fixed-size ring buffer for values of type T with the specified capacity.
// If capacity is less than or equal to zero, a capacity of 1 is used. The returned RingBuffer has
// its underlying storage allocated and is ready for use.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer[T]{
		buffer: make([]T, capacity),
	}
}

// Push adds an item, overwriting oldest if full. O(1), zero allocations.
func (r *RingBuffer[T]) Push(item T) {
	r.buffer[r.head] = item
	r.head = (r.head + 1) % len(r.buffer)
	if r.count < len(r.buffer) {
		r.count++
	}
}

// Len returns the number of valid entries
func (r *RingBuffer[T]) Len() int {
	return r.count
}

// ForEach iterates over valid entries in chronological order (oldest first)
// Return false from fn to stop iteration early
func (r *RingBuffer[T]) ForEach(fn func(*T) bool) {
	if r.count == 0 {
		return
	}
	start := 0
	if r.count == len(r.buffer) {
		start = r.head // Full buffer: oldest is at head
	}
	for i := 0; i < r.count; i++ {
		if !fn(&r.buffer[(start+i)%len(r.buffer)]) {
			break
		}
	}
}
