package memo2

import "sync"

type Func func(key string) (interface{}, error)

type result struct {
	value interface{}
	err   error
}

// A Memo caches the results of calling a Func.
type Memo struct {
	f     Func
	mu    sync.Mutex // guards cache
	cache map[string]result
}

func New(f Func) *Memo {
	return &Memo{f: f, cache: make(map[string]result)}
}

func (memo *Memo) Get(key string) (interface{}, error) {
	memo.mu.Lock()
	res, ok := memo.cache[key]
	if !ok {
		res.value, res.err = memo.f(key)
		memo.cache[key] = res
	}
	memo.mu.Unlock()
	return res.value, res.err
}
