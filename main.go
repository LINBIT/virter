package main

import (
	"github.com/LINBIT/virter/cmd"
	_ "github.com/LINBIT/virter/cmd/image"
	_ "github.com/LINBIT/virter/cmd/vm"
)

func main() {
	cmd.Execute()
}
