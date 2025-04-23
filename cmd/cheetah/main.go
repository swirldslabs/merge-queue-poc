package main

import (
	"fmt"
	"golang.hedera.com/solo-cheetah/cmd/cheetah/commands"
	"os"
)

func main() {

	err := commands.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
