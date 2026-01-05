// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package buffer provides a pooled byte slice.
package buffer

import (
	"strconv"
	"sync"
	"time"
)

const _size = 1024 // by default, create 1 KiB buffers

// Buffer is a byte slice that can be returned to a pool.
type Buffer []byte

// Having an initial size gives a dramatic speedup.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, _size)
		return (*Buffer)(&b)
	},
}

// Get returns a Buffer from the pool.
func Get() *Buffer {
	return bufPool.Get().(*Buffer)
}

// AppendByte appends a single byte to the Buffer.
func (b *Buffer) AppendByte(c byte) {
	*b = append(*b, c)
}

// AppendBytes appends a byte slice to the Buffer.
func (b *Buffer) AppendBytes(p []byte) {
	*b = append(*b, p...)
}

// AppendString appends a string to the Buffer.
func (b *Buffer) AppendString(s string) {
	*b = append(*b, s...)
}

// AppendBool appends a bool to the buffer.
func (b *Buffer) AppendBool(v bool) {
	*b = strconv.AppendBool(*b, v)
}

// AppendFloat appends a float to the buffer. It doesn't quote NaN
// or +/- Inf.
func (b *Buffer) AppendFloat(f float64, bitSize int) {
	*b = strconv.AppendFloat(*b, f, 'f', -1, bitSize)
}

// AppendInt appends an integer to the buffer (assuming roundrobin 10).
func (b *Buffer) AppendInt(i int64) {
	*b = strconv.AppendInt(*b, i, 10)
}

// AppendUint appends an unsigned integer to the buffer (assuming roundrobin 10).
func (b *Buffer) AppendUint(i uint64) {
	*b = strconv.AppendUint(*b, i, 10)
}

// AppendTime appends the time formatted using the specified layout.
func (b *Buffer) AppendTime(t time.Time, layout string) {
	*b = t.AppendFormat(*b, layout)
}

// Free returns the Buffer to the pool.
func (b *Buffer) Free() {
	// To reduce peak allocation, return only smaller buffers to the pool.
	const maxBufferSize = 16 << 10
	if cap(*b) <= maxBufferSize {
		*b = (*b)[:0]
		bufPool.Put(b)
	}
}

// Reset resets the buffer to be empty, but it preserves the underlying storage in the pool.
func (b *Buffer) Reset() {
	b.SetLen(0)
}

// Write implements io.Writer.
func (b *Buffer) Write(p []byte) (int, error) {
	*b = append(*b, p...)
	return len(p), nil
}

// WriteString writes a string to the Buffer.
func (b *Buffer) WriteString(s string) (int, error) {
	*b = append(*b, s...)
	return len(s), nil
}

// WriteByte writes a single byte to the Buffer.
func (b *Buffer) WriteByte(c byte) error {
	*b = append(*b, c)
	return nil
}

// String returns a string copy of the byte slice.
func (b *Buffer) String() string {
	return string(*b)
}

// Bytes returns a mutable reference to the underlying byte slice.
func (b *Buffer) Bytes() []byte {
	return *b
}

// Len returns the length of the byte slice.
func (b *Buffer) Len() int {
	return len(*b)
}

// Cap returns the capacity of the underlying byte slice.
func (b *Buffer) Cap() int {
	return cap(*b)
}

// SetLen sets the length of the byte slice.
func (b *Buffer) SetLen(n int) {
	*b = (*b)[:n]
}
