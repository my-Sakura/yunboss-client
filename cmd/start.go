package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/my-Sakura/zinx/api"
	"github.com/my-Sakura/zinx/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	_apiGroup = ""
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start msgclient",
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			config = &client.Config{}
			log    = logrus.New()
		)
		if err := viper.Unmarshal(config); err != nil {
			return err
		}
		client.LogPath = config.LogPath
		log.AddHook(&client.MyHook{})

		gin.SetMode(gin.DebugMode)
		var file *os.File
		defer file.Close()
		y, month, d := time.Now().Date()
		fileName := filepath.Join(config.LogPath, fmt.Sprintf("%d-%d-%d.log", y, int(month), d))
		_, err := os.Stat(fileName)
		if err != nil {
			if os.IsNotExist(err) {
				file, err = os.Create(fileName)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			file, err = os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, os.ModePerm)
			if err != nil {
				return err
			}
		}
		fi, err := file.Stat()
		if err != nil {
			return err
		}
		if fi.Mode() != os.ModePerm {
			if err = file.Chmod(os.ModePerm); err != nil {
				return err
			}
		}
		gin.DefaultWriter = io.MultiWriter(os.Stdout, file)

		engine := gin.Default()
		engine.Use(api.Cors())

		client := client.New(config, log)
		m := api.New(client)
		m.Regist(engine.Group(_apiGroup))

		go client.Start()
		if err := engine.Run(":" + client.Config.Apiport); err != nil {
			return err
		}

		return nil
	},
}
