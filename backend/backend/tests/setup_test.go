package tests

import (
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Transcendence/config"
	"github.com/Transcendence/models"
	"github.com/Transcendence/redis"
	"github.com/Transcendence/routes"
)

// SetupTestEnv prepares a clean test environment:
//   - drops ALL tables (users, friends, posts, files, notifications, etc.)
//   - re-runs AutoMigrate to recreate them
//   - returns a fresh router and DB handle
//
// Call this at the start of each test that talks to the DB.
func SetupTestEnv() (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_NAME", "app_db")

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	db, err := config.ConnectDB()
	if err != nil {
		panic(err)
	}

	rdb, err := redis.InitRedis()
	if err != nil {
		panic(err)
	}

	// Drop all tables in a single statement so foreign keys don't get in the way
	db.Exec(`
		DROP TABLE IF EXISTS
			file_accesses,
			files,
			notifications,
			messages,
			reposts,
			replies,
			likes,
			posts,
			friends,
			users
		CASCADE
	`)

	// Recreate all tables via AutoMigrate
	if err := db.AutoMigrate(
		&models.User{},
		&models.Friend{},
		&models.Post{},
		&models.Like{},
		&models.Reply{},
		&models.Repost{},
		&models.Message{},
		&models.Notification{},
		&models.File{},
		&models.FileAccess{},
	); err != nil {
		panic(err)
	}

	// Clear Redis to avoid pollution between tests (rate limits, pending tokens, etc.)
	rdb.FlushDB(rdb.Context())

	router := gin.Default()
	routes.SetupRoutes(router, db, rdb, cfg)

	return router, db
}
