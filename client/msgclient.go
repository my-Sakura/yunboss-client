package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Client struct {
	Token          string
	Config         *Config
	ClientReturnCh chan *ClientReturnBody
	ServerPushCh   chan *ServerPushBody
	Done           chan struct{}
	Conn           net.Conn
}

type HttpHeartBeatBody []struct {
	IP   string `json:"ip"`
	UID  string `json:"uid"`
	Body struct {
		Process struct {
			Nginx int `json:"nginx"`
			Php   int `json:"php"`
			Mysql int `json:"mysql"`
		} `json:"process"`
		HTTP struct {
			Disk int `json:"disk"`
		} `json:"http"`
		Shell struct {
			Network string `json:"network"`
		} `json:"shell"`
	} `json:"body"`
}

func New() *Client {
	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		panic(err)
	}

	return &Client{
		Config:         config,
		Done:           make(chan struct{}),
		ClientReturnCh: make(chan *ClientReturnBody),
		ServerPushCh:   make(chan *ServerPushBody),
	}
}

func (c *Client) cmd(port string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Error listen: %s", err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accept client: %s", err.Error())
			continue
		}

		go c.HandleCmd(conn)
	}
}

func (c *Client) HandleCmd(conn net.Conn) {
	defer conn.Close()
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
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

			fmt.Println("config reload succeed")
			return

		case "status":
			if _, err := conn.Write([]byte("msgclient running")); err != nil {
				panic(err)
			}
			return

		default:
			fmt.Println("debug")
		}
	}
}

func (c *Client) Start() error {
	conn, err := net.Dial("tcp", c.Config.Msgservice)
	if err != nil {
		return err
	}
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
			log.Println(err)
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
				fmt.Println("repeated login")
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
		if loginBody.Status != http.StatusOK {
			if loginBody.Status == http.StatusConflict {
				log.Fatalf("Error login: %s", "repeated uid")
			} else {
				time.Sleep(time.Second * 10)
				continue
			}
		} else {
			c.Token = loginBody.Token
			fmt.Println("loginBody", loginBody)
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
			log.Printf("status: %s, msg: %s\n", heartBeatBody.Status, heartBeatBody.Msg)

		case "clientpush":
			clientPush := &ClientReturnBody{}
			if err = json.Unmarshal(buf[:n], clientPush); err != nil {
				panic(err)
			}
			c.ClientReturnCh <- clientPush

		case "serverpush":
			serverPush := &ServerPushBody{}
			if err = json.Unmarshal(buf[:n], serverPush); err != nil {
				panic(err)
			}
			c.ServerPushCh <- serverPush

		default:
			fmt.Println("debug")
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
			log.Println(err)
		}
	}()
	ticker := time.NewTicker(time.Second * time.Duration(c.Config.HeartBeat))

	for {
		select {
		case <-ticker.C:
			// httpHeartBeatBody := HttpHeartBeatBody{}
			if c.Config.Monitor == "" {
				fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
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
				fmt.Println("HTTP Post timeout")
				continue
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}

			// if err = json.Unmarshal(body, &httpHeartBeatBody); err != nil {
			// 	panic(err)
			// }

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
