package models

import "time"

// Label represents shipping information assigned to packages
type Label struct {
	ID          string    `gorm:"column:label_id;primaryKey;type:varchar(50)"`
	PackageID   string    `gorm:"column:package_id;type:varchar(50);index;not null"`
	Package     *Package  `gorm:"foreignKey:PackageID"`
	SessionID   string    `gorm:"column:session_id;type:varchar(50);index;not null"`
	Session     *Session  `gorm:"foreignKey:SessionID"`
	Destination string    `gorm:"column:destination;type:varchar(255);not null"`
	CourierID   string    `gorm:"column:courier_id;type:varchar(50);index"`
	Courier     string    `gorm:"foreignKey:CourierID"`
	Priority    string    `gorm:"column:priority;type:varchar(20);default:'standard'"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}
