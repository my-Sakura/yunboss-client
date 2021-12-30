package cmd

import (
	"github.com/gin-gonic/gin"
	"github.com/my-Sakura/zinx/api"
	"github.com/my-Sakura/zinx/msgclient"
	"github.com/spf13/cobra"
)

const (
	_apiGroup   = ""
	_clientAddr = "0.0.0.0:5000"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start msgclient",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := gin.Default()
		engine.Use(api.Cors())

		client := msgclient.New()
		m := api.New(client)
		m.Regist(engine.Group(_apiGroup))

		go client.Start()
		if err := engine.Run(_clientAddr); err != nil {
			return err
		}

		return nil
	},
}
