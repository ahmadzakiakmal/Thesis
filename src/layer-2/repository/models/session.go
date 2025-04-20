package models

import "time"

// Session represents a workflow instance containing multiple operations
type Session struct {
	ID          string    `gorm:"column:session_id;primaryKey;type:varchar(50)"`
	Status      string    `gorm:"column:status;type:varchar(20);not null"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
	OperatorID  string    `gorm:"column:operator_id;type:varchar(50);index"`
	Operator    *Operator `gorm:"foreignKey:OperatorID"`
	IsCommitted bool      `gorm:"column:is_committed;default:false"`
	TxHash      *string   `gorm:"column:tx_hash;type:varchar(66)"` // Null if not committed
	Package     *Package  `gorm:"foreignKey:SessionID"`

	// Relationships
	QCRecords   []QCRecord   `gorm:"foreignKey:SessionID"`
	Labels      []Label      `gorm:"foreignKey:SessionID"`
	Transaction *Transaction `gorm:"foreignKey:SessionID"`
}
