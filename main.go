/*
Copyright © 2023 Dagu Yota Hamada
*/
package main

import (
	cmd "github.com/yohamta/dagu/cmd"
	"github.com/yohamta/dagu/internal/constants"
)

func main() {
	cmd.Execute()
}

var version = "0.0.0"

func init() {
	constants.Version = version
}
