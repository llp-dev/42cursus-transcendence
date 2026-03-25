package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()

	if (err != nil) {
		log.Println("[WARNING] .env file not found")
	}

	var router *gin.Engine= gin.Default()
	router.SetTrustedProxies(nil)

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			// gin.H = map[string]any{}
			"message": "Backend API is running",
			"status": "success",
		})
	})

	port := os.Getenv("API_PORT")
	if (port == "") {
		port = ":8000"
	}

	router.Run(":" + port)
}
