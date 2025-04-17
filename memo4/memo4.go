package memo4

import "sync"

type Func func(key string) (interface{}, error)

type result struct {
	value interface{}
	err   error
}

type entry struct {
	res   result
	ready chan struct{} // closed when res is ready
}

/*
This implementation of Memo uses a mutex to guard a map variable
that is shared by each goroutine that calls Get().
*/
type Memo struct {
	f     Func
	mu    sync.Mutex // guards cache
	cache map[string]*entry
}

func New(f Func) *Memo {
	return &Memo{f: f, cache: make(map[string]*entry)}
}

func (memo *Memo) Get(key string) (interface{}, error) {
	memo.mu.Lock()
	e := memo.cache[key]
	if e == nil {
		// This is the first request for this key
		// This goroutine becomes reponsible for computing
		// the value and broadcasting the ready condition
		e = &entry{ready: make(chan struct{})}
		memo.cache[key] = e
		memo.mu.Unlock()

		e.res.value, e.res.err = memo.f(key)
		close(e.ready) // broadcast ready condition
	} else {
		// This is a repeat request for this key
		memo.mu.Unlock()
		<-e.ready // wait for ready condition
	}

	return e.res.value, e.res.err
}
