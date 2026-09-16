package config

import (
	"crypto/rand"
	"flag"
	"log"
	"os"
)

type Config struct {
	ServerListenAddr   string
	ServerBaseUrl      string
	FileStoragePath    string
	LogDirectory       string
	DbConnectionString string
	SecretKey          []byte
}

func InitConfig() Config {
	config := Config{}

	setConfigFromArgs(&config)
	setConfigFromEnv(&config)

	if len(config.SecretKey) == 0 {
		secretKey, err := generateSecretKey(32)
		if err != nil {
			log.Fatalf("Can't generate secret key. Error: %s", err.Error())
		}
		config.SecretKey = secretKey
	}

	return config
}

func setConfigFromArgs(config *Config) {
	const defaultListen = ":8080"
	const defaultUrl = "http://localhost:8080"
	const defaultFileStoragePath = ""
	const defaultLogDirectory = ""
	const defaultDbConnection = ""

	secretKey := ""

	flag.StringVar(&config.ServerListenAddr, "a", defaultListen, `Address to run server. Default: ":8080"`)
	flag.StringVar(&config.ServerBaseUrl, "b", defaultUrl, `Base address of the resulting shortened URL. Default: "http://localhost:8080"`)
	flag.StringVar(&config.FileStoragePath, "f", defaultFileStoragePath, `File path to storage file. Default: ""`)
	flag.StringVar(&config.LogDirectory, "l", defaultLogDirectory, `Log directory. Default puts messages to stdout`)
	flag.StringVar(&config.DbConnectionString, "d", defaultDbConnection, `Database connection string. Default: ""`)
	flag.StringVar(&secretKey, "k", "", `Secret key string. Default: ""`)
	flag.Parse()

	config.SecretKey = []byte(secretKey)
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

	if logDirectory, found := os.LookupEnv("LOG_LOCATION_PATH"); found {
		config.LogDirectory = logDirectory
	}

	if dbConnectionString, found := os.LookupEnv("DATABASE_DSN"); found {
		config.DbConnectionString = dbConnectionString
	}

	if secretKeyString, found := os.LookupEnv("SHORTENER_SECRET_KEY"); found {
		config.SecretKey = []byte(secretKeyString)
	}
}

func generateSecretKey(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
