package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/my-Sakura/zinx/msgclient"
)

type Manager struct {
	client *msgclient.Client
}

func New(client *msgclient.Client) *Manager {
	return &Manager{
		client: client,
	}
}

func (m *Manager) Regist(r gin.IRouter) {
	r.POST("/sendmsg", m.sendMsg)
}

func (m *Manager) sendMsg(c *gin.Context) {
	var req struct {
		Body string `json:"body" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": 1, "msg": err.Error()})
		log.Printf("Error bind request: %v\n", err)
		return
	}

	if err := m.client.SendMsg(req.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 1, "msg": err.Error()})
		log.Printf("Error sendMsg: %v\n", err)
		return
	}
	receiveData := <-m.client.ClientPushCh

	c.JSON(http.StatusOK, gin.H{"status": 0, "msg": receiveData.Body})
}
