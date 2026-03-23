package controler

import (
	"github.com/Lord-Lucius/Transcendence/entity"
	"github.com/Lord-Lucius/Transcendence/service"
	"github.com/gin-gonic/gin"
)

type UserController interface {
	FindAll() []entity.User
	Save(ctx *gin.Context)
}
type controller struct {
	service service.UserService
}

func New(service service.UserService) UserController {
	return controller{
		service: service,
	}
}

func (c *controller) FindAll() []entity.User {
	return service.FindAll()
}

func (c *controller) Save(ctx *gin.Context) {

}
