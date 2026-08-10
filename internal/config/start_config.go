package config

import (
	"flag"
	"os"
)

type Config struct {
	ServerListenAddr string
	ServerBaseUrl    string
}

func InitConfig() Config {
	config := Config{}

	const defaultListen = ":8080"
	const defaultUrl = "http://localhost:8080"

	var needToParseArgs bool

	if listenAddr, found := os.LookupEnv("SERVER_ADDRESS"); found {
		config.ServerListenAddr = listenAddr
	} else {
		flag.StringVar(&config.ServerListenAddr, "a", defaultListen, `Address to run server. Default: ":8080"`)
		needToParseArgs = true
	}

	if urlAddr, found := os.LookupEnv("BASE_URL"); found {
		config.ServerBaseUrl = urlAddr
	} else {
		flag.StringVar(&config.ServerBaseUrl, "b", defaultUrl, `Base address of the resulting shortened URL. Default: "http://localhost:8080"`)
		needToParseArgs = true
	}

	if needToParseArgs {
		flag.Parse()
	}

	return config
}
