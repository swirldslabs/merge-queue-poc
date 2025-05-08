package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/logx"
)

var (
	// Used for flags.
	flagConfig string

	rootCmd = &cobra.Command{
		Use:   "cheetah",
		Short: "A fast and efficient Stream files uploader",
		Long:  "Solo Cheetah - A fast and efficient stream file uploader",
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "d", "", "config file path")

	// make flags mandatory
	_ = rootCmd.MarkPersistentFlagRequired("config")
	//_ = rootCmd.MarkPersistentFlagRequired("host-id")
	//_ = rootCmd.MarkPersistentFlagRequired("start-date")
	//_ = rootCmd.MarkPersistentFlagRequired("end-date")

	rootCmd.AddCommand(uploadCmd)
}

func initConfig() {
	var err error
	err = config.Initialize(flagConfig)
	if err != nil {
		fmt.Println("failed to initialize config")
		cobra.CheckErr(err)
	}

	err = logx.Initialize(config.Get().Log)
	if err != nil {
		fmt.Println(err)
		cobra.CheckErr(err)
	}

}
