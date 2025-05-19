package commands

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"golang.hedera.com/solo-cheetah/pkg/sniff"
	_ "net/http/pprof"
)

var (
	// Used for flags.
	flagConfig string
	flagPoll   bool // exit after execution

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

	rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&flagPoll, "poll", "", true, "poll for marker files")

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

func startProfiling(ctx context.Context) error {
	profilingConf := *config.Get().Profiling
	return sniff.Start(ctx, profilingConf)
}
