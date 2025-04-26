package models

import "time"

// Transaction represents blockchain/ledger records of committed sessions
type Transaction struct {
	TxHash      string    `gorm:"column:tx_hash;type:varchar(66)"`
	SessionID   string    `gorm:"column:session_id;type:varchar(50);uniqueIndex;not null;primaryKey"`
	Session     *Session  `gorm:"foreignKey:SessionID"`
	BlockHeight int64     `gorm:"column:block_height;not null"`
	Timestamp   time.Time `gorm:"column:timestamp;not null"`
	Status      string    `gorm:"column:status;type:varchar(20);default:'confirmed'"`
}
