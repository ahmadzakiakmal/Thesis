package models

import "time"

// QCRecord represents quality control inspection results
type QCRecord struct {
	ID          string    `gorm:"column:qc_id;primaryKey;type:varchar(50)"`
	PackageID   string    `gorm:"column:package_id;type:varchar(50);index;not null"`
	Package     *Package  `gorm:"foreignKey:PackageID"`
	SessionID   string    `gorm:"column:session_id;type:varchar(50);index;not null"`
	Session     *Session  `gorm:"foreignKey:SessionID"`
	Passed      bool      `gorm:"column:passed;not null"`
	InspectorID string    `gorm:"column:inspector_id;type:varchar(50);index"`
	Inspector   *Operator `gorm:"foreignKey:InspectorID"`
	Issues      string    `gorm:"column:issues;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}
