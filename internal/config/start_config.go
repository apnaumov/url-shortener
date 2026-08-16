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

	const defaultListen = ":8080"
	const defaultUrl = "http://localhost:8080"
	const defaultFileStoragePath = "url_storage.storage"

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

	if storageFilePath, found := os.LookupEnv("FILE_STORAGE_PATH"); found {
		config.FileStoragePath = storageFilePath
	} else {
		flag.StringVar(&config.FileStoragePath, "f", defaultFileStoragePath, `File path to storage file. Default: "$(pwd)/url_storage.storage"`)
		needToParseArgs = true
	}

	if needToParseArgs {
		flag.Parse()
	}

	return config
}
