package client

type Config struct {
	Uid        string
	HeartBeat  int
	Msgservice string
	Boss       string
	LogPath    string `mapstructure:"log_path"`
	Monitor    string
	Apiport    string
	Port       string
}
