package msgclient

type Config struct {
	Uid        string
	HeartBeat  int
	Msgservice string
	Boss       string
	LogPath    string `mapstructure:"log_path"`
	Monitor    string
}
