package client

type ServerLoginBody struct {
	Type   string `json:"type"`
	Ip     string `json:"ip"`
	UID    string `json:"uid"`
	Body   string `json:"body"`
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Token  string `json:"token"`
}

type ServerHeartBeatBody struct {
	Ip     string `json:"ip"`
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

type ServerPushBody struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
	Body string `json:"body"`
	URL  string `json:"url"`
}

type ServerReturnBody struct {
	Type   string `json:"type"`
	Body   string `json:"body"`
	Msg    string `json:"msg"`
	Status string `json:"status"`
}
