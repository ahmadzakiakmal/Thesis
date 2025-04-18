package database

import (
	"log"
	"time"

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
