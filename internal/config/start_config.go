package config

import (
	"flag"
	"os"
)

type Config struct {
	ServerListenAddr string
	ServerBaseUrl    string
	FileStoragePath  string
}

func InitConfig() Config {
	config := Config{}

	setConfigFromArgs(&config)
	setConfigFromEnv(&config)

	return config
}

func setConfigFromArgs(config *Config) {
	const defaultListen = ":8080"
	const defaultUrl = "http://localhost:8080"
	const defaultFileStoragePath = "url_storage.storage"

	flag.StringVar(&config.ServerListenAddr, "a", defaultListen, `Address to run server. Default: ":8080"`)
	flag.StringVar(&config.ServerBaseUrl, "b", defaultUrl, `Base address of the resulting shortened URL. Default: "http://localhost:8080"`)
	flag.StringVar(&config.FileStoragePath, "f", defaultFileStoragePath, `File path to storage file. Default: "$(pwd)/url_storage.storage"`)
	flag.Parse()
}

func setConfigFromEnv(config *Config) {
	if listenAddr, found := os.LookupEnv("SERVER_ADDRESS"); found {
		config.ServerListenAddr = listenAddr
	}

	if urlAddr, found := os.LookupEnv("BASE_URL"); found {
		config.ServerBaseUrl = urlAddr
	}

	if storageFilePath, found := os.LookupEnv("FILE_STORAGE_PATH"); found {
		config.FileStoragePath = storageFilePath
	}
}
