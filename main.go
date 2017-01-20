package main

import (
	"github.com/reechou/wxrobot/config"
	"github.com/reechou/wxrobot/logic"
)

func main() {
	logic.NewWxLogic(config.NewConfig()).Run()
}
