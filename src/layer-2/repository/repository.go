package repository

import (
	"log"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgreSQL error codes as constants
const (
	// Class 23 — Integrity Constraint Violation
	PgErrForeignKeyViolation = "23503" // foreign_key_violation
	PgErrUniqueViolation     = "23505" // unique_violation
	PgErrCheckViolation      = "23514" // check_violation
	PgErrNotNullViolation    = "23502" // not_null_violation
	PgErrExclusionViolation  = "23P01" // exclusion_violation

	// Class 22 — Data Exception
	PgErrDataException          = "22000" // data_exception
	PgErrNumericValueOutOfRange = "22003" // numeric_value_out_of_range
	PgErrInvalidDatetimeFormat  = "22007" // invalid_datetime_format
	PgErrDivisionByZero         = "22012" // division_by_zero

	// Class 42 — Syntax Error or Access Rule Violation
	PgErrSyntaxError           = "42601" // syntax_error
	PgErrInsufficientPrivilege = "42501" // insufficient_privilege
	PgErrUndefinedTable        = "42P01" // undefined_table
	PgErrUndefinedColumn       = "42703" // undefined_column

	// Class 08 — Connection Exception
	PgErrConnectionException = "08000" // connection_exception
	PgErrConnectionFailure   = "08006" // connection_failure

	// Class 40 — Transaction Rollback
	PgErrTransactionRollback                     = "40000" // transaction_rollback
	PgErrTransactionIntegrityConstraintViolation = "40002" // transaction_integrity_constraint_violation

	// Class 53 — Insufficient Resources
	PgErrInsufficientResources = "53000" // insufficient_resources
	PgErrDiskFull              = "53100" // disk_full
	PgErrOutOfMemory           = "53200" // out_of_memory

	// Class 57 — Operator Intervention
	PgErrAdminShutdown = "57P01" // admin_shutdown
	PgErrCrashShutdown = "57P02" // crash_shutdown

	// Class 58 — System Error
	PgErrIOError = "58030" // io_error

	// Class XX — Internal Error
	PgErrInternalError = "XX000" // internal_error
)

type Repository struct {
	DB  *gorm.DB
	dsn string
}

func NewDatabaseService(dsn string) *Repository {
	return &Repository{
		DB:  nil,
		dsn: dsn,
	}
}

func (r *Repository) Connect() {
	for i := range 10 {
		log.Printf("Connection attempt %d...\n", i+1)
		DB, err := gorm.Open(postgres.Open(r.dsn))
		if err != nil {
			log.Printf("Connection attempt %d, failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
		r.DB = DB
	}
}

func (r *Repository) Migrate() {
	// Migrate existing User model
	r.DB.AutoMigrate(&models.User{})

	// Migrate all supply chain system models
	r.DB.AutoMigrate(
		&models.User{}, // This one is for testing only
		&models.Courier{},
		&models.Session{},
		&models.Operator{},
		&models.Supplier{},
		&models.Package{},
		&models.Item{},
		&models.ItemCatalog{},
		&models.QCRecord{},
		&models.Label{},
		&models.Transaction{},
	)

	// Log migration completion
	log.Println("Database migration completed successfully")
}

func (r *Repository) Seed() {
	// Check if data already exists to avoid duplicates
	var supplierCount int64
	r.DB.Model(&models.Supplier{}).Count(&supplierCount)

	if supplierCount > 0 {
		log.Println("Seed data already exists, skipping...")
		return
	}

	log.Println("Seeding database with initial data...")

	// Create suppliers
	suppliers := []models.Supplier{
		{ID: "SUP-001", Name: "Global Distribution Co.", Location: "Singapore"},
		{ID: "SUP-002", Name: "East Asia Logistics", Location: "Hong Kong"},
		{ID: "SUP-003", Name: "Prime Warehouse Solutions", Location: "Jakarta"},
		{ID: "SUP-004", Name: "Quality Goods Inc.", Location: "Kuala Lumpur"},
		{ID: "SUP-005", Name: "Regional Supply Chain", Location: "Bangkok"},
	}

	for _, supplier := range suppliers {
		if err := r.DB.Create(&supplier).Error; err != nil {
			log.Printf("Error creating supplier %s: %v", supplier.ID, err)
		}
	}

	// Create operators
	operators := []models.Operator{
		{ID: "OPR-001", Name: "John Smith", Role: "Warehouse Manager", AccessLevel: "Admin"},
		{ID: "OPR-002", Name: "Sarah Lee", Role: "Quality Control Specialist", AccessLevel: "Standard"},
		{ID: "OPR-003", Name: "Raj Patel", Role: "Logistics Coordinator", AccessLevel: "Standard"},
		{ID: "OPR-004", Name: "Maria Garcia", Role: "Inventory Clerk", AccessLevel: "Basic"},
		{ID: "OPR-005", Name: "David Wong", Role: "Shipping Specialist", AccessLevel: "Standard"},
		{ID: "OPR-006", Name: "Lisa Chen", Role: "Receiving Clerk", AccessLevel: "Basic"},
	}

	for _, operator := range operators {
		if err := r.DB.Create(&operator).Error; err != nil {
			log.Printf("Error creating operator %s: %v", operator.ID, err)
		}
	}

	// Create item catalog
	catalogItems := []models.ItemCatalog{
		{ID: "CAT-001", Name: "Smartphone Model X", Description: "Latest flagship smartphone", Category: "Electronics", UnitWeight: 0.2, UnitValue: 899.99},
		{ID: "CAT-002", Name: "Wireless Earbuds", Description: "Noise-cancelling earbuds", Category: "Electronics", UnitWeight: 0.05, UnitValue: 149.99},
		{ID: "CAT-003", Name: "Tablet Pro", Description: "12-inch professional tablet", Category: "Electronics", UnitWeight: 0.6, UnitValue: 1299.99},
		{ID: "CAT-004", Name: "Smart Watch", Description: "Health monitoring smartwatch", Category: "Electronics", UnitWeight: 0.1, UnitValue: 249.99},
		{ID: "CAT-005", Name: "Bluetooth Speaker", Description: "Waterproof portable speaker", Category: "Electronics", UnitWeight: 0.3, UnitValue: 79.99},
		{ID: "CAT-006", Name: "USB-C Cable", Description: "2m braided charging cable", Category: "Accessories", UnitWeight: 0.05, UnitValue: 19.99},
		{ID: "CAT-007", Name: "Laptop Sleeve", Description: "15-inch protective sleeve", Category: "Accessories", UnitWeight: 0.2, UnitValue: 29.99},
		{ID: "CAT-008", Name: "Power Bank", Description: "20000mAh fast charging", Category: "Electronics", UnitWeight: 0.4, UnitValue: 59.99},
	}

	for _, item := range catalogItems {
		if err := r.DB.Create(&item).Error; err != nil {
			log.Printf("Error creating catalog item %s: %v", item.ID, err)
		}
	}

	// Create couriers
	couriers := []models.Courier{
		{ID: "COU-001", Name: "Speedy Express", ServiceLevel: "Premium", ContactInfo: "support@speedyexpress.com"},
		{ID: "COU-002", Name: "Global Logistics", ServiceLevel: "Standard", ContactInfo: "cs@globallogistics.com"},
		{ID: "COU-003", Name: "Asia Direct", ServiceLevel: "Economy", ContactInfo: "help@asiadirect.com"},
		{ID: "COU-004", Name: "Swift Cargo", ServiceLevel: "Same-day", ContactInfo: "service@swiftcargo.com"},
		{ID: "COU-005", Name: "Pacific Shipping", ServiceLevel: "Standard", ContactInfo: "info@pacificshipping.com"},
	}

	for _, courier := range couriers {
		if err := r.DB.Create(&courier).Error; err != nil {
			log.Printf("Error creating courier %s: %v", courier.ID, err)
		}
	}

	log.Println("Database seeding completed successfully")
}
