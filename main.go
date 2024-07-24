/*
Copyright © 2023 Dagu Yota Hamada
*/
package main

import (
	"os"

	"github.com/daguflow/dagu/cmd"
	"github.com/daguflow/dagu/internal/constants"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}

}

var version = "0.0.0"

func init() {
	constants.Version = version
}
