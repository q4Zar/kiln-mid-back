package tzkt

import "time"

type DelegationResponse struct {
	ID           int64     `json:"id"`
	Level        int64     `json:"level"`
	Timestamp    time.Time `json:"timestamp"`
	Block        string    `json:"block"`
	Hash         string    `json:"hash"`
	Sender       Sender    `json:"sender"`
	NewDelegate  *Delegate `json:"newDelegate"`
	Amount       int64     `json:"amount"`
	PrevDelegate *Delegate `json:"prevDelegate"`
	Status       string    `json:"status"`
}

type Sender struct {
	Address string `json:"address"`
	Alias   string `json:"alias,omitempty"`
}

type Delegate struct {
	Address string `json:"address"`
	Alias   string `json:"alias,omitempty"`
}

type QueryParams struct {
	Limit     int
	Offset    int
	Level     *LevelFilter
	Timestamp *TimestampFilter
	Sort      []string
	Select    []string
}

type LevelFilter struct {
	Eq  *int64
	Ne  *int64
	Gt  *int64
	Gte *int64
	Lt  *int64
	Lte *int64
	In  []int64
	Ni  []int64
}

type TimestampFilter struct {
	Eq  *time.Time
	Ne  *time.Time
	Gt  *time.Time
	Gte *time.Time
	Lt  *time.Time
	Lte *time.Time
}
