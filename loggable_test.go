package loggable_test

import (
	"fmt"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"testing"
	"time"

	loggable "github.com/sas1024/gorm-loggable"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

type SomeType struct {
	gorm.Model
	Source string
	MetaModel
}

type MetaModel struct {
	createdBy string
	loggable.LoggableModel
}

func (m MetaModel) Meta() interface{} {
	return struct {
		CreatedBy string
	}{CreatedBy: m.createdBy}
}

func TestMain(m *testing.M) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second,   // 慢 SQL 阈值
			LogLevel:      logger.Info,   // Log level
			Colorful:      false,         // 禁用彩色打印
		},
	)
	//dsn := "user=postgres password=Nimo%40123 DB.name=loggable port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		"postgres",
		"Nimo%40123",
		"localhost",
		5432,
		"loggable",
	)
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: newLogger})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	_, err = loggable.Register(database)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	err = database.AutoMigrate(SomeType{})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	db = database
	os.Exit(m.Run())
}

func TestTryModel(t *testing.T) {
	newmodel := SomeType{Source: time.Now().Format(time.Stamp)}
	newmodel.createdBy = "some user"
	err := db.Create(&newmodel).Error
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(newmodel.ID)
	newmodel.Source = "updated field"
	err = db.Save(&newmodel).Error
	if err != nil {
		t.Fatal(err)
	}
	var st SomeType

	// 可以
	db.First(&st)
	fmt.Println(st)
}
