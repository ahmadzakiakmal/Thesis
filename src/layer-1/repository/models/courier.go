package models

// Courier represents shipping service providers
type Courier struct {
	ID           string `gorm:"column:courier_id;primaryKey;type:varchar(50)"`
	Name         string `gorm:"column:name;type:varchar(100);not null"`
	ServiceLevel string `gorm:"column:service_level;type:varchar(50)"`
	ContactInfo  string `gorm:"column:contact_info;type:varchar(255)"`

	// Relationships
	Labels []Label `gorm:"foreignKey:CourierID"`
}
