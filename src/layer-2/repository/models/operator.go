package models

// Operator represents users who perform actions in the system
type Operator struct {
	ID          string `gorm:"column:operator_id;primaryKey;type:varchar(50)"`
	Name        string `gorm:"column:name;type:varchar(100);not null"`
	Role        string `gorm:"column:role;type:varchar(50)"`
	AccessLevel string `gorm:"column:access_level;type:varchar(20);default:'Basic'"`

	// Relationships
	Sessions []Session `gorm:"foreignKey:OperatorID"`
}
