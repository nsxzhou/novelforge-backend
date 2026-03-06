package main

import (
	"flag"
	"log"

	"novelforge/backend/internal/app"
)

func main() {
	// 解析命令行参数：`-config` 指定配置文件路径，默认 `configs/config.yam
	configPath := flag.String("config", "configs/config.yaml", "path to runtime config")
	flag.Parse()

	// 加载配置
	bootstrap, err := app.LoadBootstrap(*configPath)
	if err != nil {
		log.Fatalf("bootstrap service: %v", err)
	}

	// 启动 Hertz HTTP 服务
	bootstrap.HTTP.Spin()
}
