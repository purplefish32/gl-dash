package main

import (
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/purplefish32/gl-dash/cmd"
)

func main() {
	zone.NewGlobal()
	cmd.Execute()
}
