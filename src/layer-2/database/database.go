package database

import (
	"log"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/database/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseService struct {
	DB  *gorm.DB
	dsn string
}

func NewDatabaseService(dsn string) *DatabaseService {
	return &DatabaseService{
		DB:  nil,
		dsn: dsn,
	}
}

func (dbSvc *DatabaseService) Connect() {
	for i := range 10 {
		log.Printf("Connection attempt %d...\n", i+1)
		DB, err := gorm.Open(postgres.Open(dbSvc.dsn))
		if err != nil {
			log.Printf("Connection attempt %d, failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
		dbSvc.DB = DB
	}
}

func (dbSvc *DatabaseService) Migrate() {
	// Migrate existing User model
	dbSvc.DB.AutoMigrate(&models.User{})

	// Migrate all supply chain system models
	dbSvc.DB.AutoMigrate(
		&models.User{}, // This one is for testing only
		&models.Session{},
		&models.Operator{},
		&models.Supplier{},
		&models.Package{},
		&models.Item{},
		&models.ItemCatalog{},
		&models.QCRecord{},
		&models.Label{},
		&models.Courier{},
		&models.Transaction{},
	)

	// Log migration completion
	log.Println("Database migration completed successfully")
}
