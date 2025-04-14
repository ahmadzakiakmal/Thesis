package models

type User struct {
	Name  string `json:"name"`
	Email string `json:"email" gorm:"primaryKey;unique"`
	// Email string `json:"email"`
}
