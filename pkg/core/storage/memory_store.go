package storage

import (
	"strings"
	"sync"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
	mut sync.RWMutex
	mem map[string][]byte
	// A map, not a slice, to avoid duplicates.
	del map[string]bool
}

// MemoryBatch a in-memory batch compatible with MemoryStore.
type MemoryBatch struct {
	m map[string][]byte
	// A map, not a slice, to avoid duplicates.
	del map[string]bool
}

// Put implements the Batch interface.
func (b *MemoryBatch) Put(k, v []byte) {
	vcopy := make([]byte, len(v))
	copy(vcopy, v)
	kcopy := string(k)
	b.m[kcopy] = vcopy
	delete(b.del, kcopy)
}

// Delete implements Batch interface.
func (b *MemoryBatch) Delete(k []byte) {
	kcopy := string(k)
	delete(b.m, kcopy)
	b.del[kcopy] = true
}

// NewMemoryStore creates a new MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		mem: make(map[string][]byte),
		del: make(map[string]bool),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	if val, ok := s.mem[string(key)]; ok {
		return val, nil
	}
	return nil, ErrKeyNotFound
}

// Put implements the Store interface. Never returns an error.
func (s *MemoryStore) Put(key, value []byte) error {
	s.mut.Lock()
	newKey := string(key)
	s.mem[newKey] = value
	delete(s.del, newKey)
	s.mut.Unlock()
	return nil
}

// Delete implements Store interface. Never returns an error.
func (s *MemoryStore) Delete(key []byte) error {
	s.mut.Lock()
	newKey := string(key)
	s.del[newKey] = true
	delete(s.mem, newKey)
	s.mut.Unlock()
	return nil
}

// PutBatch implements the Store interface. Never returns an error.
func (s *MemoryStore) PutBatch(batch Batch) error {
	b := batch.(*MemoryBatch)
	for k := range b.del {
		_ = s.Delete([]byte(k))
	}
	for k, v := range b.m {
		_ = s.Put([]byte(k), v)
	}
	return nil
}

// Seek implements the Store interface.
func (s *MemoryStore) Seek(key []byte, f func(k, v []byte)) {
	for k, v := range s.mem {
		if strings.HasPrefix(k, string(key)) {
			f([]byte(k), v)
		}
	}
}

// Batch implements the Batch interface and returns a compatible Batch.
func (s *MemoryStore) Batch() Batch {
	return newMemoryBatch()
}

// newMemoryBatch returns new memory batch.
func newMemoryBatch() *MemoryBatch {
	return &MemoryBatch{
		m:   make(map[string][]byte),
		del: make(map[string]bool),
	}
}

// Persist flushes all the MemoryStore contents into the (supposedly) persistent
// store provided via parameter.
func (s *MemoryStore) Persist(ps Store) (int, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	batch := ps.Batch()
	keys, dkeys := 0, 0
	for k, v := range s.mem {
		batch.Put([]byte(k), v)
		keys++
	}
	for k := range s.del {
		batch.Delete([]byte(k))
		dkeys++
	}
	var err error
	if keys != 0 || dkeys != 0 {
		err = ps.PutBatch(batch)
	}
	if err == nil {
		s.mem = make(map[string][]byte)
		s.del = make(map[string]bool)
	}
	return keys, err
}

// Close implements Store interface and clears up memory. Never returns an
// error.
func (s *MemoryStore) Close() error {
	s.mut.Lock()
	s.del = nil
	s.mem = nil
	s.mut.Unlock()
	return nil
}
