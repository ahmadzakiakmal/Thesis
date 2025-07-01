package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository/models"
	cmtrpc "github.com/cometbft/cometbft/rpc/client/local"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/jackc/pgx/v5/pgconn"
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

// ConsensusPayload represents data that will be sent to consensus
type ConsensusPayload interface{}

// ConsensusResult contains the result of a consensus operation
type ConsensusResult struct {
	TxHash      string
	BlockHeight int64
	Code        uint32
	Error       error
}

// RepositoryError represent an error in the repository layer (db/rpc)
type RepositoryError struct {
	Code    string
	Message string
	Detail  string
}

type Repository struct {
	db          *gorm.DB
	rpcClient   *cmtrpc.Local
	l1Addresses []string
	httpClient  *http.Client
}

func NewRepository() *Repository {
	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}
	return &Repository{
		httpClient: &httpClient,
	}
}

func (r *Repository) ConnectDB(dsn string) {
	for i := range 10 {
		log.Printf("Connection attempt %d...\n", i+1)
		DB, err := gorm.Open(postgres.Open(dsn))
		if err != nil {
			log.Printf("Connection attempt %d, failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
		r.db = DB
		log.Println("Connected to Postgres")
	}
}

func (r *Repository) Migrate() {
	// Migrate existing User model
	r.db.AutoMigrate(&models.User{})

	// Migrate all supply chain system models
	r.db.AutoMigrate(
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
	r.db.Model(&models.Supplier{}).Count(&supplierCount)

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
		if err := r.db.Create(&supplier).Error; err != nil {
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
		if err := r.db.Create(&operator).Error; err != nil {
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
		if err := r.db.Create(&item).Error; err != nil {
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
		if err := r.db.Create(&courier).Error; err != nil {
			log.Printf("Error creating courier %s: %v", courier.ID, err)
		}
	}

	// Create sample packages with items
	packages := []models.Package{
		{ID: "PKG-001", SupplierID: "SUP-001", DeliveryNoteID: "DN-001", Signature: "digital_sig_001", IsTrusted: false},
		{ID: "PKG-002", SupplierID: "SUP-002", DeliveryNoteID: "DN-002", Signature: "digital_sig_002", IsTrusted: false},
		{ID: "PKG-003", SupplierID: "SUP-003", DeliveryNoteID: "DN-003", Signature: "digital_sig_003", IsTrusted: false},
		{ID: "PKG-004", SupplierID: "SUP-001", DeliveryNoteID: "DN-004", Signature: "digital_sig_004", IsTrusted: false},
		{ID: "PKG-005", SupplierID: "SUP-004", DeliveryNoteID: "DN-005", Signature: "digital_sig_005", IsTrusted: false},
	}

	for _, pkg := range packages {
		if err := r.db.Create(&pkg).Error; err != nil {
			log.Printf("Error creating package %s: %v", pkg.ID, err)
		}
	}

	// Create items for each package
	items := []models.Item{
		{ID: "ITEM-001", PackageID: "PKG-001", Quantity: 5, Description: "Smartphones", CatalogID: ptrString("CAT-001")},
		{ID: "ITEM-002", PackageID: "PKG-001", Quantity: 10, Description: "Earbuds", CatalogID: ptrString("CAT-002")},
		{ID: "ITEM-003", PackageID: "PKG-002", Quantity: 3, Description: "Tablets", CatalogID: ptrString("CAT-003")},
		{ID: "ITEM-004", PackageID: "PKG-002", Quantity: 8, Description: "Watches", CatalogID: ptrString("CAT-004")},
		{ID: "ITEM-005", PackageID: "PKG-003", Quantity: 15, Description: "Speakers", CatalogID: ptrString("CAT-005")},
		{ID: "ITEM-006", PackageID: "PKG-003", Quantity: 50, Description: "Cables", CatalogID: ptrString("CAT-006")},
		{ID: "ITEM-007", PackageID: "PKG-004", Quantity: 20, Description: "Laptop Sleeves", CatalogID: ptrString("CAT-007")},
		{ID: "ITEM-008", PackageID: "PKG-004", Quantity: 12, Description: "Power Banks", CatalogID: ptrString("CAT-008")},
		{ID: "ITEM-009", PackageID: "PKG-005", Quantity: 4, Description: "Tablets", CatalogID: ptrString("CAT-003")},
		{ID: "ITEM-010", PackageID: "PKG-005", Quantity: 25, Description: "Cables", CatalogID: ptrString("CAT-006")},
	}

	for _, item := range items {
		if err := r.db.Create(&item).Error; err != nil {
			log.Printf("Error creating item %s: %v", item.ID, err)
		}
	}

	log.Println("Database seeding completed successfully")
}

func (r *Repository) SetupRpcClient(rpcClient *cmtrpc.Local, l1Addresses []string) {
	r.rpcClient = rpcClient
	r.l1Addresses = l1Addresses
}

func ptrString(s string) *string {
	return &s
}

// DB Operations

// CreateSession creates a new session in the Database
func (r *Repository) CreateSession(
	sessionID,
	operatorID string,
) (*models.Session, *RepositoryError) {
	session := models.Session{
		ID:          sessionID,
		Status:      "active",
		IsCommitted: false,
		OperatorID:  operatorID,
	}

	dbTx := r.db.Begin()
	err := dbTx.Create(&session).Error
	if err != nil {
		dbTx.Rollback()
		pgErr, isPgError := err.(*pgconn.PgError)
		if isPgError {
			fmt.Println(pgErr.Code)
			return nil, &RepositoryError{
				Code:    string(pgErr.Code),
				Message: pgErr.Message,
				Detail:  pgErr.Detail,
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error occured",
			Detail:  err.Error(),
		}
	}

	err = dbTx.Commit().Error
	if err != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error occured",
			Detail:  err.Error(),
		}
	}

	return &session, nil
}

// ScanPackage returns the expected item and signature of the package
func (r *Repository) ScanPackage(sessionID, packageID string) (*models.Package, *RepositoryError) {
	// Begin transaction
	dbTx := r.db.Begin()

	// Find the package by ID with preloaded items and supplier
	var pkg models.Package
	err := dbTx.Preload("Items").Preload("Supplier").Where("package_id = ?", packageID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Package does not exist",
				Detail:  fmt.Sprintf("Package with id %s does not exist", packageID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	pkg.Status = "pending_validation"

	// Save changes
	err = dbTx.Save(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package",
			Detail:  err.Error(),
		}
	}

	// Commit transaction
	err = dbTx.Commit().Error
	if err != nil {
		return nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return &pkg, nil
}

// ValidatePackage validates the supplier's signature, links package to the session
func (r *Repository) ValidatePackage(supplierSignature, packageID, SessionID string) (*models.Package, *RepositoryError) {
	dbTx := r.db.Begin()

	var pkg models.Package
	err := dbTx.Preload("Items").Preload("Supplier").Where("package_id = ?", packageID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Package does not exist",
				Detail:  fmt.Sprintf("Package with id %s does not exist", packageID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// * For this PoC, assume all signature is valid
	pkg.IsTrusted = true
	pkg.SessionID = &SessionID
	pkg.Status = "validated"

	err = dbTx.Save(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package",
			Detail:  err.Error(),
		}
	}

	err = dbTx.Commit().Error
	if err != nil {
		return nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return &pkg, nil
}

// QualityCheck adds a QC Record to a Package
func (r *Repository) QualityCheck(sessionID string, qcPassed bool, issues []string) (*models.Package, *models.QCRecord, *RepositoryError) {
	dbTx := r.db.Begin()

	var session models.Session
	err := dbTx.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Session does not exist",
				Detail:  fmt.Sprintf("Session with id %s does not exist", sessionID),
			}
		}
		return nil, nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Find the package by ID
	var pkg models.Package
	err = dbTx.Preload("Items").Where("session_id = ?", sessionID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Package does not exist",
				Detail:  fmt.Sprintf("Package associated with session %s does not exist", sessionID),
			}
		}
		return nil, nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Ensure package status is appropriate for QC
	if pkg.Status != "validated" {
		dbTx.Rollback()
		return nil, nil, &RepositoryError{
			Code:    "INVALID_STATE",
			Message: "Package is not ready for QC",
			Detail:  fmt.Sprintf("Package status is %s, must be 'validated'", pkg.Status),
		}
	}

	// Convert issues slice to string for storage
	issuesStr := ""
	if len(issues) > 0 {
		issuesBytes, err := json.Marshal(issues)
		if err != nil {
			dbTx.Rollback()
			return nil, nil, &RepositoryError{
				Code:    "MARSHALING_ERROR",
				Message: "Failed to process issues data",
				Detail:  err.Error(),
			}
		}
		issuesStr = string(issuesBytes)
	}

	// Generate QC record ID
	compositeID := pkg.ID + sessionID
	hash := sha256.Sum256([]byte(compositeID))
	qcID := fmt.Sprintf("QC-%s", hex.EncodeToString(hash[:])[:16])

	// Create QC record
	qcRecord := models.QCRecord{
		ID:          qcID,
		PackageID:   pkg.ID,
		SessionID:   sessionID,
		Passed:      qcPassed,
		InspectorID: session.OperatorID,
		Issues:      issuesStr,
	}

	// Save QC record
	err = dbTx.Create(&qcRecord).Error
	if err != nil {
		dbTx.Rollback()
		return nil, nil, &RepositoryError{
			Code:    "INSERT_FAILED",
			Message: "Failed to create QC record",
			Detail:  err.Error(),
		}
	}

	// Update package status based on QC result
	if qcPassed {
		pkg.Status = "qc_passed"
	} else {
		pkg.Status = "qc_failed"
	}

	// Save updated package status
	err = dbTx.Save(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		return nil, nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package status",
			Detail:  err.Error(),
		}
	}

	// Commit transaction
	err = dbTx.Commit().Error
	if err != nil {
		return nil, nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return &pkg, &qcRecord, nil
}

// LabelPackage adds a label to the package
func (r *Repository) LabelPackage(sessionID, destination, priority, courierID string) (*models.Label, *RepositoryError) {
	dbTx := r.db.Begin()

	log.Printf("Creating label for session: %s, destination: %s, priority: %s, courierID: %s",
		sessionID, destination, priority, courierID)

	var session models.Session
	err := dbTx.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		dbTx.Rollback()
		log.Printf("Session lookup error: %v", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Session does not exist",
				Detail:  fmt.Sprintf("Session with id %s does not exist", sessionID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "A database error occurred",
			Detail:  err.Error(),
		}
	}
	log.Printf("Found session: %s", session.ID)

	var pkg models.Package
	err = dbTx.Where("session_id = ?", sessionID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		log.Printf("Package lookup error: %v", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "No package found for this session",
				Detail:  fmt.Sprintf("No package found for session %s", sessionID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "A database error occurred",
			Detail:  err.Error(),
		}
	}
	log.Printf("Found package: %s for session: %s", pkg.ID, sessionID)

	var courier models.Courier
	err = dbTx.Where("courier_id = ?", courierID).First(&courier).Error
	if err != nil {
		dbTx.Rollback()
		log.Printf("Courier lookup error: %v", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Courier does not exist",
				Detail:  fmt.Sprintf("Courier with id %s does not exist", courierID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "A database error occurred",
			Detail:  err.Error(),
		}
	}

	hash := sha256.Sum256([]byte(courier.ID + pkg.ID + sessionID))
	labelID := fmt.Sprintf("LBL-%s", hex.EncodeToString(hash[:])[:16])
	log.Printf("Generated label ID: %s", labelID)

	newLabel := models.Label{
		ID:          labelID,
		PackageID:   pkg.ID,
		SessionID:   session.ID,
		Destination: destination,
		CourierID:   courier.ID,
		Courier:     courier.Name,
		Priority:    priority,
	}

	dbTx = dbTx.Debug()

	err = dbTx.Create(&newLabel).Error
	if err != nil {
		dbTx.Rollback()
		log.Printf("Failed to create label: %v", err)
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to create label",
			Detail:  err.Error(),
		}
	}

	var checkLabel models.Label
	err = dbTx.Where("label_id = ?", labelID).First(&checkLabel).Error
	if err != nil {
		dbTx.Rollback()
		log.Printf("Failed to verify label creation: %v", err)
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Label created but couldn't be verified",
			Detail:  err.Error(),
		}
	}
	log.Printf("Label verified in transaction: %s", checkLabel.ID)

	err = dbTx.Commit().Error
	if err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	log.Printf("Transaction committed successfully, label created: %s", labelID)

	var finalCheck models.Label
	err = r.db.Where("label_id = ?", labelID).First(&finalCheck).Error
	if err != nil {
		log.Printf("WARNING: Label not found after commit: %v", err)
	} else {
		log.Printf("CONFIRMED: Label %s exists in database", finalCheck.ID)
	}

	return &newLabel, nil
}

// CommitSession commits the session to L1
func (r *Repository) CommitSession(sessionID, operatorID string) (*models.Transaction, *RepositoryError) {
	dbTx := r.db.Begin()

	var session models.Session
	err := dbTx.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "ENTITY_NOT_FOUND",
				Message: "Session does not exist",
				Detail:  fmt.Sprintf("Session with id %s does not exist", sessionID),
			}
		}
		pgErr, isPgError := err.(*pgconn.PgError)
		if isPgError {
			return nil, &RepositoryError{
				Code:    pgErr.Code,
				Detail:  pgErr.Detail,
				Message: pgErr.Message,
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "a database error occured",
			Detail:  err.Error(),
		}
	}

	if session.Status == "committed" {
		return nil, &RepositoryError{
			Code:    "CONFLICT",
			Message: "Session already committed",
			Detail:  "the session is already committed",
		}
	}

	if operatorID != session.OperatorID {
		return nil, &RepositoryError{
			Code:    "UNAUTHORIZED",
			Message: "Authorization Failed",
			Detail:  "you are not authorized to commit this session",
		}
	}

	var pkg *models.Package
	err = dbTx.Preload("Items").Preload("QCRecords").Preload("QCRecords.Inspector").Where("session_id = ?", session.ID).First(&pkg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "INVALID_STATE",
				Message: "Session not ready for commit",
				Detail:  fmt.Sprintf("Session %s does not have a package associated to it", sessionID),
			}
		}
		pgErr, isPgError := err.(*pgconn.PgError)
		if isPgError {
			return nil, &RepositoryError{
				Code:    pgErr.Code,
				Detail:  pgErr.Detail,
				Message: pgErr.Message,
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "a database error occured",
			Detail:  err.Error(),
		}
	}
	if pkg.Status != "qc_passed" {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "INVALID_STATE",
			Message: "Package not ready for commit",
			Detail:  fmt.Sprintf("Package status is %s, must be 'qc_passed'", pkg.Status),
		}
	}
	var label models.Label
	err = dbTx.Where("package_id = ?", pkg.ID).First(&label).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "INVALID_STATE",
				Message: "Package not labeled",
				Detail:  "Package must be labeled before committing",
			}
		}
		return nil, &RepositoryError{
			Code:    "INVALID_STATE",
			Message: "Package not ready for commit",
			Detail:  fmt.Sprintf("Package %s, is not yet labelled", pkg.ID),
		}
	}

	qcPassed := pkg.Status == "qc_passed"
	rawIssues := pkg.QCRecords[0].Issues
	rawIssues = rawIssues[1 : len(rawIssues)-1]
	issues := strings.Split(rawIssues, ",")

	l1Resp, repoErr := r.CommitToL1(session.ID, session.OperatorID, pkg.ID, "signature123", label.Destination, label.Priority, label.CourierID, qcPassed, issues)
	if repoErr != nil {
		dbTx.Rollback()
		return nil, repoErr
	}

	tx := models.Transaction{
		SessionID:   session.ID,
		BlockHeight: l1Resp.BlockHeight,
		Status:      "committed",
	}
	session.IsCommitted = true
	session.Status = "committed"
	session.TxHash = &tx.TxHash // Assuming TxHash field exists in Session model

	err = dbTx.Save(&session).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to update session status",
			Detail:  err.Error(),
		}
	}
	pkg.Status = "committed"
	err = dbTx.Save(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to update package status",
			Detail:  err.Error(),
		}
	}
	err = dbTx.Create(&tx).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "CONSENSUS_ERROR",
			Message: "a consensus error occurred",
			Detail:  err.Error(),
		}
	}

	err = dbTx.Commit().Error
	if err != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to commit database transaction",
			Detail:  err.Error(),
		}
	}
	return &tx, nil
}

// RunConsensus handles submitting data to the blockchain and waiting for consensus. For L2, this runs a consensus simulation
func (r *Repository) RunConsensus(ctx context.Context, payload ConsensusPayload) (*ConsensusResult, *RepositoryError) {
	// Serialize the payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, &RepositoryError{
			Code:    "SERIALIZATION_ERROR",
			Message: "Failed to serialize consensus payload",
			Detail:  err.Error(),
		}
	}

	// Create consensus transaction
	consensusTx := cmttypes.Tx(payloadBytes)

	// Use a channel to detect both context deadline and RPC completion
	done := make(chan struct {
		result *cmtrpctypes.ResultBroadcastTxCommit
		err    error
	}, 1)

	go func() {
		result, err := r.rpcClient.BroadcastTxCommit(ctx, consensusTx)
		done <- struct {
			result *cmtrpctypes.ResultBroadcastTxCommit
			err    error
		}{result, err}
	}()

	// Wait for either the operation to complete or context to be canceled
	select {
	case <-ctx.Done():
		return nil, &RepositoryError{
			Code:    "CONSENSUS_TIMEOUT",
			Message: "Consensus operation timed out",
			Detail:  ctx.Err().Error(),
		}
	case result := <-done:
		if result.err != nil {
			return nil, &RepositoryError{
				Code:    "CONSENSUS_ERROR",
				Message: "Failed to commit to blockchain",
				Detail:  result.err.Error(),
			}
		}

		// Check for errors in the response
		if result.result.CheckTx.Code != 0 {
			return nil, &RepositoryError{
				Code:    "CONSENSUS_ERROR",
				Message: "Blockchain rejected transaction",
				Detail:  fmt.Sprintf("CheckTx code: %d", result.result.CheckTx.Code),
			}
		}

		// Return success result
		return &ConsensusResult{
			TxHash:      hex.EncodeToString(result.result.Hash),
			BlockHeight: result.result.Height,
			Code:        result.result.CheckTx.Code,
		}, nil
	}
}

func (r *Repository) CommitToL1(sessionID, operatorID, packageID, supplierSignature, destination, priority, courierID string, qcPassed bool, issues []string) (*ConsensusResult, *RepositoryError) {
	if len(r.l1Addresses) == 0 {
		return nil, &RepositoryError{
			Code:    "CONFIG_ERROR",
			Message: "No L1 node addresses configured",
			Detail:  "l1Addresses slice is empty",
		}
	}

	payload := map[string]interface{}{
		"session_id":         sessionID,
		"operator_id":        operatorID,
		"package_id":         packageID,
		"supplier_signature": supplierSignature,
		"destination":        destination,
		"priority":           priority,
		"courier_id":         courierID,
		"qc_passed":          qcPassed,
		"issues":             issues,
		"timestamp":          time.Now(),
		// "origin_node_id": r.node.ID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, &RepositoryError{
			Code:    "SERIALIZATION_ERROR",
			Message: "Failed to serialize L1 commit payload",
			Detail:  err.Error(),
		}
	}

	fmt.Println(r.l1Addresses[0])
	url := fmt.Sprintf("http://%s/session/%s/commit-l1", r.l1Addresses[0], sessionID)
	fmt.Println(url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, &RepositoryError{
			Code:    "REQUEST_ERROR",
			Message: "Failed to create HTTP request",
			Detail:  err.Error(),
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, &RepositoryError{
			Code:    "NETWORK_ERROR",
			Message: "Failed to connect to L1 node",
			Detail:  err.Error(),
		}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RepositoryError{
			Code:    "READ_ERROR",
			Message: "Failed to read response from L1 node",
			Detail:  err.Error(),
		}
	}

	// Or unmarshal it to a struct if it's JSON
	type TransactionStatus struct {
		TxID        string    `json:"tx_id"`
		RequestID   string    `json:"request_id"`
		Status      string    `json:"status"`
		BlockHeight int64     `json:"block_height"`
		BlockHash   string    `json:"block_hash,omitempty"`
		ConfirmTime time.Time `json:"confirm_time"`
	}
	type ClientResponse struct {
		StatusCode int               `json:"-"` // Not included in JSON
		Headers    map[string]string `json:"-"` // Not included in JSON
		// Body          string            `json:"body,omitempty"`
		Body          interface{}       `json:"body"`
		Meta          TransactionStatus `json:"meta"`
		BlockchainRef string            `json:"blockchain_ref"`
		NodeID        string            `json:"node_id"`
	}
	var result ClientResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, &RepositoryError{
			Code:    "PARSE_ERROR",
			Message: "Failed to parse L1 node response",
			Detail:  err.Error(),
		}
	}

	return &ConsensusResult{
		BlockHeight: result.Meta.BlockHeight,
		TxHash:      result.Meta.BlockHash,
		Code:        0,
	}, nil
}

// CreateTestPackage is used to create packages (for testing only)
func (r *Repository) CreateTestPackage(
	requestID string,
) (string, *RepositoryError) {
	supplierID := "SUP-001"
	pkgID := fmt.Sprintf("PKG-%s", requestID[:8])

	// Begin transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return "", &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to start transaction",
			Detail:  tx.Error.Error(),
		}
	}

	// Create package
	pkg := models.Package{
		ID:             pkgID,
		SupplierID:     supplierID,
		DeliveryNoteID: "DN-001",
		Signature:      "any",
		IsTrusted:      false,
	}

	if err := tx.Create(&pkg).Error; err != nil {
		tx.Rollback()
		return "", &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "a database error occurred",
			Detail:  err.Error(),
		}
	}

	// Add a single item to the package (minimum requirement)
	item := models.Item{
		ID:          fmt.Sprintf("ITEM-%s", requestID[len(requestID)-6:]),
		PackageID:   pkgID,
		Quantity:    1,
		Description: "Test Item",
		CatalogID:   ptrString("CAT-001"), // Assuming CAT-001 exists from seed
	}

	if err := tx.Create(&item).Error; err != nil {
		tx.Rollback()
		return "", &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to create item",
			Detail:  err.Error(),
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return "", &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return pkg.ID, nil
}
