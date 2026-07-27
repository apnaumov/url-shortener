package config

import "flag"

type Config struct {
	ServerListenAddr string
	ServerBaseUrl    string
}

func InitConfigFromCLI() Config {
	config := Config{}

	const defaultListen = ":8080"
	const defaultUrl = "http://localhost:8080"

	flag.StringVar(&config.ServerListenAddr, "a", defaultListen, `Address to run server. Default: ":8080"`)
	flag.StringVar(&config.ServerBaseUrl, "b", defaultUrl, `Base address of the resulting shortened URL. Default: "http://localhost:8080"`)

	flag.Parse()

	return config
}
