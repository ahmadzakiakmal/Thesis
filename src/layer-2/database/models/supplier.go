package models

// Supplier represents the source of packages
type Supplier struct {
	ID       string `gorm:"column:supplier_id;primaryKey;type:varchar(50)"`
	Name     string `gorm:"column:name;type:varchar(100);not null"`
	Location string `gorm:"column:location;type:varchar(100)"`

	// Relationships
	Packages []Package `gorm:"foreignKey:SupplierID"`
}
