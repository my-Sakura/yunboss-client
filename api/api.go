package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/my-Sakura/zinx/client"
)

type Manager struct {
	client *client.Client
}

func New(client *client.Client) *Manager {
	return &Manager{
		client: client,
	}
}

func (m *Manager) Regist(r gin.IRouter) {
	r.POST("/sendmsg", m.sendMsg)
}

func (m *Manager) sendMsg(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
		Body  string `json:"body" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": 1, "msg": "bad request"})
		log.Printf("Error bind request: %v\n", err)
		return
	}

	if req.Token != m.client.Token {
		c.JSON(http.StatusBadRequest, gin.H{"status": 1, "msg": "please input the right token"})
		log.Println("Error wrong token")
		return
	}
	if err := m.client.SendMsg(req.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 1, "msg": "internal error"})
		log.Printf("Error sendMsg: %v\n", err)
		return
	}
	receiveData := <-m.client.ClientReturnCh

	c.JSON(http.StatusOK, gin.H{"status": receiveData.Status, "msg": receiveData.Msg})
}
