/**
 * @Author: dingQingHui
 * @Description:
 * @File: safeslice
 * @Version: 1.0.0
 * @Date: 2025/1/2 15:31
 */

package concurrentslice

import (
	"sync"
)

func New[T any](cap int) *ConcurrentSlice[T] {
	s := new(ConcurrentSlice[T])
	s.slice = make([]T, 0, cap)
	return s
}

// ConcurrentSlice 封装了一个并发安全的泛型切片
type ConcurrentSlice[T any] struct {
	mu    sync.RWMutex
	slice []T
}

// Append 方法用于向切片中添加元素
func (s *ConcurrentSlice[T]) Append(element T) {
	s.mu.Lock()
	s.slice = append(s.slice, element)
	s.mu.Unlock()
}

// Get 方法用于获取切片中指定索引位置的元素
func (s *ConcurrentSlice[T]) Get(index int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if index < 0 || index >= len(s.slice) {
		var zero T
		return zero, false
	}
	return s.slice[index], true
}

func (s *ConcurrentSlice[T]) Range(f func(i int, element T) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, t := range s.slice {
		if !f(i, t) {
			return
		}
	}
}

// Len 方法用于获取切片的长度
func (s *ConcurrentSlice[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.slice)
}
