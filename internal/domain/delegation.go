package domain

import (
	"time"
)

type Delegation struct {
	ID            string    `json:"-" db:"id"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	Amount        string    `json:"amount" db:"amount"`
	Delegator     string    `json:"delegator" db:"delegator"`
	Level         string    `json:"level" db:"level"`
	BlockHash     string    `json:"-" db:"block_hash"`
	OperationHash string    `json:"operation_hash" db:"operation_hash"`
	CreatedAt     time.Time `json:"-" db:"created_at"`
}

type DelegationResponse struct {
	Data []Delegation `json:"data"`
}

type DelegationRepository interface {
	Save(delegation *Delegation) error
	SaveBatch(delegations []Delegation) error
	FindAll(year *int) ([]Delegation, error)
	GetLastIndexedLevel() (int64, error)
	Exists(delegator string, level string) (bool, error)
}

type DelegationService interface {
	GetDelegations(year *int) ([]Delegation, error)
	IndexDelegations(fromLevel int64) error
	StartPolling() error
	StopPolling()
}
