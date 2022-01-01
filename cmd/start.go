package cmd

import (
	"github.com/gin-gonic/gin"
	"github.com/my-Sakura/zinx/api"
	"github.com/my-Sakura/zinx/client"
	"github.com/spf13/cobra"
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
		engine := gin.Default()
		engine.Use(api.Cors())

		client := client.New()
		m := api.New(client)
		m.Regist(engine.Group(_apiGroup))

		go client.Start()
		if err := engine.Run(":" + client.Config.Apiport); err != nil {
			return err
		}

		return nil
	},
}
