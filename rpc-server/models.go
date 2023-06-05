package main

import (
	"errors"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ChatMessage struct {
	ChatID   string `gorm:"index:chat_lookup_idx,priority:1"`
	Sender   string
	Receiver string
	Text     string
	SentAt   uint64 `gorm:"index:chat_lookup_idx,priority:2"`
}

type ChatCursorCache struct {
	ChatID  string `gorm:"primaryKey"`
	Reverse bool   `gorm:"primaryKey"`
	Cursor  int64  `gorm:"primaryKey"`
	SentAt  uint64
}

type dbContextKeyType string

var (
	DBInstanceContextKey = dbContextKeyType("DB_INSTANCE")
)

func (msg *ChatMessage) ToResponse() *rpc.Message {
	return &rpc.Message{
		Chat:     msg.ChatID,
		Text:     msg.Text,
		Sender:   msg.Sender,
		SendTime: int64(msg.SentAt),
	}
}

func (msg *ChatMessage) ToChatCursor(reverse bool, cursor int64) *ChatCursorCache {
	return &ChatCursorCache{
		ChatID:  msg.ChatID,
		Reverse: reverse,
		Cursor:  cursor,
		SentAt:  msg.SentAt,
	}
}

func createChatCursorCache(msg *ChatMessage, reverse bool, cursor int64) {
	chatCursorCache := msg.ToChatCursor(reverse, cursor)
	if err := GetDatabase().Create(chatCursorCache).Error; err != nil {
		var perr *pgconn.PgError
		errors.As(err, &perr)
		if perr != nil && perr.Code != "23505" {
			log.Printf("Error when creating chat cursor cache: %+v\n", err)
		}
	}
}

var databaseConn *gorm.DB

func InitDatabase() {
	dbUrl := constructDatabaseURL()
	db, err := gorm.Open(postgres.Open(dbUrl), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Panicf("Could not connect to database: %+v\n", err)
	}

	err = db.AutoMigrate(&ChatMessage{}, &ChatCursorCache{})
	if err != nil {
		log.Printf("Error migrating schemas: %+v\n", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Panicf("Error acquiring underlying SQL DB instance: %+v\n", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)
	databaseConn = db
}

func InitTestDatabase(dialer gorm.Dialector) {
	db, err := gorm.Open(dialer, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Panicf("Could not connect to database: %+v\n", err)
	}

	err = db.AutoMigrate(&ChatMessage{}, &ChatCursorCache{})
	if err != nil {
		log.Printf("Error migrating schemas: %+v\n", err)
	}

	databaseConn = db
}

func CloseDatabase() {
	db := GetDatabase()
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}

func GetDatabase() *gorm.DB {
	if databaseConn == nil {
		panic("Database instance not initialised!")
	}
	return databaseConn
}

func constructDatabaseURL() string {
	dbUser := os.Getenv("POSTGRES_USER")
	if dbUser == "" {
		dbUser = "imservice"
	}

	dbPass := os.Getenv("POSTGRES_PASSWORD")
	if dbPass == "" {
		dbPass = "password"
	}

	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "imservice"
	}

	dbHost := os.Getenv("POSTGRES_HOST")
	if dbHost == "" {
		dbHost = "db"
	}

	dbPort := os.Getenv("POSTGRES_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUrl := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(dbUser, dbPass),
		Host:   dbHost + ":" + dbPort,
		Path:   dbName,
	}

	return dbUrl.String()
}
