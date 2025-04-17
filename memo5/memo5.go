package memo5

type Func func(key string) (interface{}, error)

type result struct {
	value interface{}
	err   error
}

type entry struct {
	res   result
	ready chan struct{} // closed when res is ready
}

// A request is a message requesting that the Func be applied to key.
type request struct {
	key      string
	response chan<- result // the client wants a single result
}

type Memo struct {
	// through which the caller of Get communites with the monitor goroutine
	requests chan request
}

func New(f Func) *Memo {
	memo := &Memo{requests: make(chan request)}
	go memo.server(f)
	return memo
}

func (memo *Memo) Get(key string) (interface{}, error) {
	response := make(chan result)
	memo.requests <- request{key, response}
	res := <-response
	return res.value, res.err
}

func (memo *Memo) Close() {
	close(memo.requests)
}

/*
The cache variable is confined to the monitor goroutine (*Memo).server.
The monitor reads requests in a loop until the request channel is closed.
For each request, it consults the cache, creating and inserting a new entry
if none was found.
*/
func (memo *Memo) server(f Func) {
	cache := make(map[string]*entry)
	for request := range memo.requests {
		e := cache[request.key]
		if e == nil {
			// This is the first request for this key.
			e = &entry{ready: make(chan struct{})}
			cache[request.key] = e
			go e.call(f, request.key) // call f(key)
		}
		go e.deliver(request.response)
	}
}

func (e *entry) call(f Func, key string) {
	// evaluate the function
	e.res.value, e.res.err = f(key)
	// broadcast the ready condition
	close(e.ready)
}

func (e *entry) deliver(response chan<- result) {
	// wait for the ready condition
	<-e.ready
	// send the result to the client
	response <- e.res
}
