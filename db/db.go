package db

import (
	"sync"
)

type BucketTest struct {
	Exist      bool
	Public     bool
	StatusCode int
	Region     string
}

type Store struct {
	mu      sync.RWMutex
	buckets map[string]BucketTest
}

func Init() (*Store, error) {
	store := &Store{
		buckets: make(map[string]BucketTest),
	}

	return store, nil
}

func Save(store *Store, url string, result BucketTest) {
	if store == nil {
		return
	}

	store.mu.Lock()
	store.buckets[url] = result
	store.mu.Unlock()
}

func Get(store *Store, url string) (BucketTest, bool) {
	if store == nil {
		return BucketTest{}, false
	}

	store.mu.RLock()
	res, ok := store.buckets[url]
	store.mu.RUnlock()

	if !ok {
		return BucketTest{}, false
	}

	return res, true
}