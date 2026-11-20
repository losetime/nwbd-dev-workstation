// Package store 是数据访问层：手写 SQL + pgx，不做 ORM 映射。
package store

import (
	"crypto/rand"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 持有连接池。
type Store struct {
	pool *pgxpool.Pool
}

// New 包装连接池。
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool 返回底层连接池，供健康检查使用。
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// itoa 是 strconv.Itoa 的短别名，只用于拼 SQL 占位符序号。
func itoa(n int) string { return strconv.Itoa(n) }

// crockford 是 Base32（ULID 用）的字母表。
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewID 生成 26 字符的 ULID：前 10 位是毫秒时间戳，后 16 位随机。
func NewID() string {
	ms := uint64(time.Now().UnixMilli())
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		for i := range entropy {
			entropy[i] = byte(ms >> (8 * (i % 8)))
		}
	}

	var out [26]byte
	for i := 9; i >= 0; i-- {
		out[i] = crockford[ms&31]
		ms >>= 5
	}

	var acc uint32
	bits := 0
	pos := 10
	for _, b := range entropy {
		acc = (acc << 8) | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[pos] = crockford[(acc>>uint(bits))&31]
			pos++
		}
	}
	if bits > 0 && pos < 26 {
		out[pos] = crockford[(acc<<uint(5-bits))&31]
	}
	return string(out[:])
}
