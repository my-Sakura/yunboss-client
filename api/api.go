package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/my-Sakura/zinx/client"
	"github.com/sirupsen/logrus"
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
		m.client.Log.WithFields(logrus.Fields{
			"err":  err,
			"time": time.Now().Format("2006-01-02 15:04:05"),
		}).Errorln("Error bind request")
		return
	}

	if req.Token != m.client.Token {
		c.JSON(http.StatusBadRequest, gin.H{"status": 1, "msg": "please input the right token"})
		m.client.Log.WithFields(logrus.Fields{
			"time": time.Now().Format("2006-01-02 15:04:05"),
		}).Errorln("Error wrong token")
		return
	}
	if err := m.client.SendMsg(req.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 1, "msg": "internal error"})
		m.client.Log.WithFields(logrus.Fields{
			"err":  err,
			"time": time.Now().Format("2006-01-02 15:04:05"),
		}).Errorln("Error sendMsg")
		return
	}
	receiveData := <-m.client.ClientReturnCh

	c.JSON(http.StatusOK, gin.H{"status": receiveData.Status, "msg": receiveData.Msg})
}
