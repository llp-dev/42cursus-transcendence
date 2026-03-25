package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	var router *gin.Engine= gin.Default()

	router.GET("/", func(ctx *gin.Context) {
		
	})
}
