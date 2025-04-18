package models

// Item represents individual items within packages
type Item struct {
	ID          string   `gorm:"column:item_id;primaryKey;type:varchar(50)"`
	PackageID   string   `gorm:"column:package_id;type:varchar(50);index;not null"`
	Package     *Package `gorm:"foreignKey:PackageID"`
	Quantity    int      `gorm:"column:qty;not null"`
	Description string   `gorm:"column:description;type:varchar(255)"`

	// Reference to catalog item (optional)
	CatalogID   *string      `gorm:"column:catalog_id;type:varchar(50);index"`
	CatalogItem *ItemCatalog `gorm:"foreignKey:CatalogID"`
}

// ItemCatalog represents the master catalog of items
type ItemCatalog struct {
	ID          string  `gorm:"column:item_catalog_id;primaryKey;type:varchar(50)"`
	Name        string  `gorm:"column:name;type:varchar(100);not null"`
	Description string  `gorm:"column:description;type:text"`
	Category    string  `gorm:"column:category;type:varchar(50)"`
	UnitWeight  float64 `gorm:"column:unit_weight;type:decimal(10,2)"`
	UnitValue   float64 `gorm:"column:unit_value;type:decimal(12,2)"`
}
