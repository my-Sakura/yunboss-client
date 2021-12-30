package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

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
	r.GET("/login", m.login)
	r.GET("/quit", m.quit)

	r.POST("/sendmsg", m.sendMsg)
	// r.POST("/heartbeat", m.heartbeat)
}

func (m *Manager) login(c *gin.Context) {
	for {
		err := m.client.Login()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError})
			log.Printf("Error login: %v\n", err)
			return
		}
		receiveData := <-m.client.LoginCh
		if receiveData.Status != http.StatusOK {
			if receiveData.Status == http.StatusConflict {
				log.Fatalf("Error login: %s", "repeated uid")
			} else {
				time.Sleep(time.Second * 10)
				continue
			}
		} else {
			m.client.Token = receiveData.Token
			break
		}
	}

	go m.client.HeartBeat()

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK})
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

func (m *Manager) quit(c *gin.Context) {
	if err := m.client.Quit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError})
		log.Printf("Error quit: %v\n", err)
		return
	}

	receiveData := <-m.client.QuitCh
	fmt.Println(receiveData, "receiveData")

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK})
}

func (m *Manager) heartbeat(c *gin.Context) {
	m.client.Trigger <- struct{}{}
	receiveData := <-m.client.HeartBeatHttpCh
	fmt.Println(receiveData, "receiveData")

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "body": receiveData})
}
