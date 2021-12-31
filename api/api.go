package api

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/my-Sakura/zinx/msgclient"
)

var (
	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		HandshakeTimeout: 5 * time.Second,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ErrWebSocketClose = errors.New("websocket client close")
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
	r.GET("/sendmsg/ws", m.wsSendMsg)

	r.POST("/sendmsg", m.sendMsg)
}

func (m *Manager) wsSendMsg(c *gin.Context) {
	wsConn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Error upgrade  websocket: %v\n", err)
		return
	}
	defer wsConn.Close()

	var (
		req struct {
			Body string `json:"body"`
		}
		resp struct {
			Status int    `json:"status"`
			Msg    string `json:"msg"`
		}
	)

	go func() {
		for {
			if err = wsConn.ReadJSON(&req); err != nil {
				resp.Status = 1
				resp.Msg = "please input json format data"

				if websocket.ErrCloseSent.Error() == err.Error() {
					log.Printf("%v\n", ErrWebSocketClose)
					return
				}
				log.Printf("Error ws read: %v\n", err)
				if err = wsConn.WriteJSON(resp); err != nil {
					if websocket.ErrCloseSent.Error() == err.Error() {
						log.Printf("%v\n", ErrWebSocketClose)
						return
					}
					log.Printf("Error ws write: %v\n", err)
					continue
				}
				continue
			}

			if err := m.client.SendMsg(req.Body); err != nil {
				resp.Status = 1
				resp.Msg = err.Error()
				if err = wsConn.WriteJSON(resp); err != nil {
					if websocket.ErrCloseSent.Error() == err.Error() {
						log.Printf("%v\n", ErrWebSocketClose)
						return
					}
					log.Printf("Error ws write: %v\n", err)
					continue
				}
				log.Printf("Error sendMsg: %v\n", err)
				continue
			}
		}
	}()

	for {
		receiveData := <-m.client.ClientPushCh

		resp.Status = 0
		resp.Msg = receiveData.Body
		if err = wsConn.WriteJSON(resp); err != nil {
			if websocket.ErrCloseSent.Error() == err.Error() {
				log.Printf("%v\n", ErrWebSocketClose)
				return
			}
			log.Printf("Error ws write: %v\n", err)
			continue
		}
	}
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
