package main

import (
	"github.com/Lord-Lucius/Transcendence/controller"
	"github.com/Lord-Lucius/Transcendence/service"
	"github.com/gin-gonic/gin"
)

var(
	userService service.UserService = service.New()
	userController controller.UserController = controller.New(userService)
)

func main() {
	server := gin.Default()

	server.GET("/users", func(ctx *gin.Context) {
		ctx.JSON(200, userController.FindAll())
	})

	server.POST("/users", func(ctx *gin.Context) {
		ctx.JSON(200, userController.Save(ctx))
	})

	server.Run(":8000")
}
