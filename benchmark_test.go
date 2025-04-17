package main

import (
	"math/rand"
	"memcache/memo2"
	"memcache/memo4"
	"memcache/memo5"
	"strconv"
	"sync"
	"testing"
	"time"
)

// A slow function to simulate expensive computation
func slowFunc(key string) (interface{}, error) {
	time.Sleep(100 * time.Millisecond)
	return key, nil
}

// Generate a million unique keys
var keys = generateKeys(1_000_000)

func generateKeys(n int) []string {
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		keys[i] = "key_" + strconv.Itoa(i)
	}
	return keys
}

func benchmarkMemo(b *testing.B, memo interface{}) {
	var wg sync.WaitGroup
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Randomly select a key from our million keys
			key := keys[r.Intn(len(keys))]
			switch m := memo.(type) {
			case *memo2.Memo:
				m.Get(key)
			case *memo4.Memo:
				m.Get(key)
			case *memo5.Memo:
				m.Get(key)
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkMemo2(b *testing.B) {
	memo := memo2.New(slowFunc)
	benchmarkMemo(b, memo)
}

func BenchmarkMemo4(b *testing.B) {
	memo := memo4.New(slowFunc)
	benchmarkMemo(b, memo)
}

func BenchmarkMemo5(b *testing.B) {
	memo := memo5.New(slowFunc)
	defer memo.Close()
	benchmarkMemo(b, memo)
}

// Test with skewed access pattern (some keys are accessed more frequently)
func benchmarkMemoSkewedAccess(b *testing.B, memo interface{}) {
	var wg sync.WaitGroup
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 80% of requests go to 20% of keys (Pareto principle)
			var key string
			if r.Float64() < 0.8 {
				// Access hot keys (first 20% of keys)
				key = keys[r.Intn(len(keys)/5)]
			} else {
				// Access cold keys (remaining 80% of keys)
				key = keys[len(keys)/5+r.Intn(len(keys)*4/5)]
			}
			switch m := memo.(type) {
			case *memo2.Memo:
				m.Get(key)
			case *memo4.Memo:
				m.Get(key)
			case *memo5.Memo:
				m.Get(key)
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkMemo2Skewed(b *testing.B) {
	memo := memo2.New(slowFunc)
	benchmarkMemoSkewedAccess(b, memo)
}

func BenchmarkMemo4Skewed(b *testing.B) {
	memo := memo4.New(slowFunc)
	benchmarkMemoSkewedAccess(b, memo)
}

func BenchmarkMemo5Skewed(b *testing.B) {
	memo := memo5.New(slowFunc)
	defer memo.Close()
	benchmarkMemoSkewedAccess(b, memo)
}
