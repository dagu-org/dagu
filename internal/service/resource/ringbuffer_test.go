package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer_Wraparound(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](3)

	// Add 5 items - should wrap around
	for i := 1; i <= 5; i++ {
		rb.Push(i)
	}

	assert.Equal(t, 3, rb.Len())

	// Values should be in chronological order: 3, 4, 5
	var values []int
	rb.ForEach(func(v *int) bool {
		values = append(values, *v)
		return true
	})

	assert.Equal(t, []int{3, 4, 5}, values)
}

func TestRingBuffer_EarlyTermination(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](10)
	for i := 1; i <= 5; i++ {
		rb.Push(i)
	}

	var values []int
	rb.ForEach(func(v *int) bool {
		values = append(values, *v)
		return len(values) < 2 // Stop after 2
	})

	assert.Equal(t, []int{1, 2}, values)
}

func TestRingBuffer_Empty(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](5)
	assert.Equal(t, 0, rb.Len())

	var called bool
	rb.ForEach(func(_ *int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

func TestRingBuffer_InvalidCapacity(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](0)
	assert.Equal(t, 1, len(rb.buffer)) // Defaults to 1

	rb2 := NewRingBuffer[int](-5)
	assert.Equal(t, 1, len(rb2.buffer))
}

func TestRingBuffer_SingleElement(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[string](1)
	rb.Push("a")
	rb.Push("b")

	assert.Equal(t, 1, rb.Len())

	var values []string
	rb.ForEach(func(v *string) bool {
		values = append(values, *v)
		return true
	})

	assert.Equal(t, []string{"b"}, values)
}

func TestRingBuffer_PartialFill(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](10)
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)

	assert.Equal(t, 3, rb.Len())

	var values []int
	rb.ForEach(func(v *int) bool {
		values = append(values, *v)
		return true
	})

	assert.Equal(t, []int{1, 2, 3}, values)
}
