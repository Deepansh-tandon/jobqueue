package db 

import(
	"log"
	"os"
	"jobqueue/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDb(){
	dsn:= os.Getenv("POSTGRES_DSN")
	var err error
	DB , err =gorm.Open(postgres.Open(dsn),&gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	 if err := DB.AutoMigrate(&models.User{}, &models.Project{}, &models.Job{}); err != nil {
        log.Fatalf("auto-migrate failed: %v", err)
    }
}