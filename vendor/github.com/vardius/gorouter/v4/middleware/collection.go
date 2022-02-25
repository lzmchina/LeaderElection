package middleware

import (
	"sort"
)

// Collection is a slice of handler wrappers functions
type Collection []Middleware

// NewCollection provides new middleware
func NewCollection(ms ...Middleware) Collection {
	return ms
}

// Merge merges another middleware
func (c Collection) Merge(m Collection) Collection {
	return append(c, m...)
}

// Compose returns middleware composed to single WrapperFunc
func (c Collection) Compose(h Handler) Handler {
	if h == nil {
		return nil
	}

	for i := range c {
		h = c[len(c)-1-i].Wrap(h)
	}

	return h
}

// Sort sorts collection by priority
func (c Collection) Sort() Collection {
	sort.SliceStable(c, func(i, j int) bool {
		return c[i].Priority() < c[j].Priority()
	})

	return c
}
