package models

// Package represents physical packages being processed
type Package struct {
	ID             string    `gorm:"column:package_id;primaryKey;type:varchar(50)"`
	SessionID      *string   `gorm:"column:session_id;type:varchar(50);index;unique"`
	Session        *Session  `gorm:"foreignKey:SessionID"`
	SupplierID     string    `gorm:"column:supplier_id;type:varchar(50);index"`
	Supplier       *Supplier `gorm:"foreignKey:SupplierID"`
	DeliveryNoteID string    `gorm:"column:delivery_note_id;type:varchar(50)"`
	Signature      string    `gorm:"column:signature;type:text"`
	IsTrusted      bool      `gorm:"column:is_trusted;default:false"`
	Status         string    `gorm:"column:status;type:varchar(20);default:'pending'"`

	// Relationships
	Items     []Item     `gorm:"foreignKey:PackageID"`
	QCRecords []QCRecord `gorm:"foreignKey:PackageID"`
	Labels    []Label    `gorm:"foreignKey:PackageID"`
}
