package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Client struct {
	Token          string
	Config         *Config
	Log            *logrus.Logger
	ClientReturnCh chan *ClientReturnBody
	ServerPushCh   chan *ServerPushBody
	Done           chan struct{}
	Conn           net.Conn
}

func New(config *Config, log *logrus.Logger) *Client {
	return &Client{
		Config:         config,
		Log:            log,
		Done:           make(chan struct{}),
		ClientReturnCh: make(chan *ClientReturnBody),
		ServerPushCh:   make(chan *ServerPushBody),
	}
}

func (c *Client) cmd(port string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		c.Log.WithFields(logrus.Fields{
			"time": time.Now().Format("2006-01-02 15:04:05"),
		}).Fatalf("Error listen: %s\n", err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			c.Log.WithFields(logrus.Fields{
				"err":  err.Error(),
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Errorln("Error accept client")
			continue
		}

		go c.HandleCmd(conn)
	}
}

func (c *Client) HandleCmd(conn net.Conn) {
	defer conn.Close()
	defer func() {
		if err := recover(); err != nil {
			c.Log.WithFields(logrus.Fields{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Errorln(err)
		}
	}()

	for {
		var buf = make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		var readRequest struct {
			Type string `json:"type"`
		}
		if err = json.Unmarshal(buf[:n], &readRequest); err != nil {
			panic(err)
		}

		switch readRequest.Type {
		case "stop":
			fmt.Println("process exit")
			if _, err := conn.Write([]byte("msgclient exit")); err != nil {
				panic(err)
			}
			os.Exit(0)

		case "reload":
			config := &Config{}
			if err := viper.Unmarshal(config); err != nil {
				panic(err)
			}
			c.Config = config
			if _, err := conn.Write([]byte("reload succeed")); err != nil {
				panic(err)
			}

			c.Log.WithFields(logrus.Fields{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Infoln("config reload succeed")
			return

		case "status":
			if _, err := conn.Write([]byte("msgclient running")); err != nil {
				panic(err)
			}
			return

		default:
			c.Log.WithFields(logrus.Fields{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Infoln("debug")
		}
	}
}

func (c *Client) Start() error {
	conn, err := net.Dial("tcp", c.Config.Msgservice)
	if err != nil {
		c.Log.WithFields(logrus.Fields{
			"err":  err,
			"time": time.Now().Format("2006-01-02 15:04:05"),
		}).Fatalln("connect server failed")
	}
	c.Log.Infof("time: %s      listen port: {http: %s}\n", time.Now().Format("2006-01-02 15:04:05"), c.Config.Apiport)
	defer conn.Close()
	c.Conn = conn
	go c.Handler(conn)
	c.cmd(c.Config.Port)

	return nil
}

func (c *Client) Handler(conn net.Conn) {
	defer conn.Close()
	defer func() {
		if err := recover(); err != nil {
			c.Log.WithFields(logrus.Fields{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Fatalln(err)
		}
	}()

	for {
		err := c.Login()
		if err != nil {
			panic(err)
		}
		var buf = make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				c.Log.Infof("time: %s      repeated login\n", time.Now().Format("2006-01-02 15:04:05"))
				if err = c.Login(); err != nil {
					panic(err)
				}
			}
			panic(err)
		}
		loginBody := &ServerLoginBody{}
		if err = json.Unmarshal(buf[:n], loginBody); err != nil {
			panic(err)
		}

		c.Log.Infof("time: %s      login body: {type: %s, ip: %s, uid: %s, body: %s, status: %d, msg: %s, token: %s}\n",
			time.Now().Format("2006-01-02 15:04:05"), loginBody.Type, loginBody.Ip, loginBody.UID, loginBody.Body,
			loginBody.Status, loginBody.Msg, loginBody.Token)
		if loginBody.Status != http.StatusOK {
			if loginBody.Status == http.StatusConflict {
				c.Log.WithFields(logrus.Fields{
					"time": time.Now().Format("2006-01-02 15:04:05"),
				}).Fatalf("Error login: %s\n", "repeated uid")
			} else {
				time.Sleep(time.Second * 10)
				continue
			}
		} else {
			c.Token = loginBody.Token
			break
		}
	}

	go c.HeartBeat()
	go c.ReceiveMsg(conn)

	for {
		var buf = make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		var readRequest struct {
			Type string `json:"type"`
		}
		if err = json.Unmarshal(buf[:n], &readRequest); err != nil {
			panic(err)
		}

		switch readRequest.Type {
		case "heartbeat":
			heartBeatBody := &ServerHeartBeatBody{}
			if err = json.Unmarshal(buf[:n], heartBeatBody); err != nil {
				panic(err)
			}
			c.Log.Infof("time: %s      heartbeat body: {status: %s, msg: %s}\n",
				time.Now().Format("2006-01-02 15:04:05"), heartBeatBody.Status, heartBeatBody.Msg)

		case "clientpush":
			clientPush := &ClientReturnBody{}
			if err = json.Unmarshal(buf[:n], clientPush); err != nil {
				panic(err)
			}
			c.Log.Infof("time: %s      client push return body: {status: %s, msg: %s}\n",
				time.Now().Format("2006-01-02 15:04:05"), clientPush.Status, clientPush.Msg)
			c.ClientReturnCh <- clientPush

		case "serverpush":
			serverPush := &ServerPushBody{}
			if err = json.Unmarshal(buf[:n], serverPush); err != nil {
				panic(err)
			}
			c.Log.Infof("time: %s      server push body: {uid: %s, body: %s, url: %s}\n",
				time.Now().Format("2006-01-02 15:04:05"), serverPush.UID, serverPush.Body, serverPush.URL)
			c.ServerPushCh <- serverPush

		default:
			c.Log.WithFields(logrus.Fields{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Infoln("debug")
		}
	}
}

func (c *Client) Login() error {
	req := &ClientLoginBody{
		Type: "login",
		Uid:  c.Config.Uid,
		Body: "",
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := c.Conn.Write(body); err != nil {
		return err
	}

	return nil
}

func (c *Client) ReceiveMsg(conn net.Conn) {
	for {
		select {
		case receiveData := <-c.ServerPushCh:
			var req = struct {
				Body string `json:"body"`
			}{
				Body: receiveData.Body,
			}
			reqBody, err := json.Marshal(req)
			if err != nil {
				panic(err)
			}

			url := "http://" + c.Config.Boss + receiveData.URL
			reader := bytes.NewReader(reqBody)
			request, err := http.NewRequest("POST", url, reader)
			if err != nil {
				serverReturnBody := &ServerReturnBody{
					Type:   "serverpush",
					Status: "2",
					Msg:    "url error",
					Body:   "",
				}

				d, err := json.Marshal(serverReturnBody)
				if err != nil {
					panic(err)
				}
				if _, err = c.Conn.Write(d); err != nil {
					panic(err)
				}
				continue
			}
			request.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: time.Second * 30}
			resp, err := client.Do(request)
			if err != nil {
				if strings.Contains(err.Error(), "Client.Timeout exceeded") {
					serverReturnBody := &ServerReturnBody{
						Type:   "serverpush",
						Status: "2",
						Msg:    "request timeout",
						Body:   "",
					}

					d, err := json.Marshal(serverReturnBody)
					if err != nil {
						panic(err)
					}
					if _, err = c.Conn.Write(d); err != nil {
						panic(err)
					}
					continue
				}
				serverReturnBody := &ServerReturnBody{
					Type:   "serverpush",
					Status: "2",
					Msg:    "url error",
					Body:   "",
				}

				d, err := json.Marshal(serverReturnBody)
				if err != nil {
					panic(err)
				}
				if _, err = c.Conn.Write(d); err != nil {
					panic(err)
				}
				continue
			}
			defer resp.Body.Close()

			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}

			serverReturnBody := &ServerReturnBody{
				Type:   "serverpush",
				Status: "0",
				Msg:    "push succeed",
				Body:   string(respBody),
			}

			d, err := json.Marshal(serverReturnBody)
			if err != nil {
				panic(err)
			}
			if _, err = c.Conn.Write(d); err != nil {
				panic(err)
			}

		case <-c.Done:
			return
		}
	}
}

func (c *Client) HeartBeat() {
	defer func() {
		if err := recover(); err != nil {
			c.Log.WithFields(logrus.Fields{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			}).Fatalln(err)
		}
	}()
	ticker := time.NewTicker(time.Second * time.Duration(c.Config.HeartBeat))

	for {
		select {
		case <-ticker.C:
			if c.Config.Monitor == "" {
				c.Log.WithFields(logrus.Fields{
					"time": time.Now().Format("2006-01-02 15:04:05"),
				}).Errorln()
				continue
			}
			monitor := "http://" + c.Config.Monitor + "/oservice/Heartbeat/index"

			request, err := http.NewRequest("GET", monitor, nil)
			if err != nil {
				panic(err)
			}
			request.Header.Set("Content-Type", "application/json")
			client := http.Client{Timeout: time.Second * 3}
			resp, err := client.Do(request)
			if err != nil {
				c.Log.WithFields(logrus.Fields{
					"time": time.Now().Format("2006-01-02 15:04:05"),
				}).Errorln("HTTP Post timeout")
				continue
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}

			req := &ClientHeartBeatBody{
				Type:  "heartbeat",
				UID:   c.Config.Uid,
				Token: c.Token,
				Body:  string(body),
			}
			data, err := json.Marshal(req)
			if err != nil {
				panic("[heartbeat] marshal error")
			}
			if _, err := c.Conn.Write(data); err != nil {
				panic("[heartbeat] write error")
			}

		case <-c.Done:
			return
		}
	}
}

func (c *Client) SendMsg(msg string) error {
	req := &ClientPushBody{
		Type:  "clientpush",
		UID:   c.Config.Uid,
		Token: c.Token,
		Body:  msg,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err = c.Conn.Write(body); err != nil {
		return err
	}

	return nil
}
