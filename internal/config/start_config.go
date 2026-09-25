package config

import (
	"crypto/rand"
	"flag"
	"log"
	"os"
	"strconv"
)

type PendingMessageProcessorConfig struct {
	TickTime                  uint64
	BufferSize                uint64
	MaxBatchSize              int
	WorkerPoolSize            uint64
	MaxParallelInsertsToQueue uint64
}

type Config struct {
	ServerListenAddr              string
	ServerBaseURL                 string
	FileStoragePath               string
	LogDirectory                  string
	DBConnectionString            string
	SecretKey                     []byte
	PendingMessageProcessorParams PendingMessageProcessorConfig
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
	const defaultURL = "http://localhost:8080"
	const defaultFileStoragePath = ""
	const defaultLogDirectory = ""
	const defaultDBConnection = ""

	secretKey := ""

	flag.StringVar(&config.ServerListenAddr, "a", defaultListen, `Address to run server. Default: ":8080"`)
	flag.StringVar(&config.ServerBaseURL, "b", defaultURL, `Base address of the resulting shortened URL. Default: "http://localhost:8080"`)
	flag.StringVar(&config.FileStoragePath, "f", defaultFileStoragePath, `File path to storage file. Default: ""`)
	flag.StringVar(&config.LogDirectory, "l", defaultLogDirectory, `Log directory. Default puts messages to stdout`)
	flag.StringVar(&config.DBConnectionString, "d", defaultDBConnection, `Database connection string. Default: ""`)
	flag.StringVar(&secretKey, "k", "", `Secret key string. Default: ""`)
	flag.Uint64Var(&config.PendingMessageProcessorParams.TickTime, "tick", 10, "Tick time to fetch messages in seconds. Default : 10")
	flag.Uint64Var(&config.PendingMessageProcessorParams.BufferSize, "bufferSize", 1024, "Buffer size. Default: 1024")
	flag.IntVar(&config.PendingMessageProcessorParams.MaxBatchSize, "maxBatchSize", 512, "Max batch size to send messages. Default: 512")
	flag.Uint64Var(&config.PendingMessageProcessorParams.WorkerPoolSize, "workerPoolSize", 20, "Worker pool size. Default: 20")
	flag.Uint64Var(&config.PendingMessageProcessorParams.MaxParallelInsertsToQueue, "maxParallelInsertsToQueue", 20, "Maximum parallel inserts to message queue. Default: 20")
	flag.Parse()

	config.SecretKey = []byte(secretKey)
}

func setConfigFromEnv(config *Config) {
	if listenAddr, found := os.LookupEnv("SERVER_ADDRESS"); found {
		config.ServerListenAddr = listenAddr
	}

	if URLAddr, found := os.LookupEnv("BASE_URL"); found {
		config.ServerBaseURL = URLAddr
	}

	if storageFilePath, found := os.LookupEnv("FILE_STORAGE_PATH"); found {
		config.FileStoragePath = storageFilePath
	}

	if logDirectory, found := os.LookupEnv("LOG_LOCATION_PATH"); found {
		config.LogDirectory = logDirectory
	}

	if dbConnectionString, found := os.LookupEnv("DATABASE_DSN"); found {
		config.DBConnectionString = dbConnectionString
	}

	if secretKeyString, found := os.LookupEnv("SHORTENER_SECRET_KEY"); found {
		config.SecretKey = []byte(secretKeyString)
	}

	if tickTimeString, found := os.LookupEnv("SHORTENER_TICK_TIME"); found {
		u64, err := strconv.ParseUint(tickTimeString, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		config.PendingMessageProcessorParams.TickTime = u64
	}

	if bufferSizeString, found := os.LookupEnv("SHORTENER_BUFFER_SIZE"); found {
		u64, err := strconv.ParseUint(bufferSizeString, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		config.PendingMessageProcessorParams.BufferSize = u64
	}

	if maxBatchString, found := os.LookupEnv("SHORTENER_MAX_BATCH_SIZE"); found {
		i, err := strconv.Atoi(maxBatchString)
		if err != nil {
			log.Fatal(err)
		}
		config.PendingMessageProcessorParams.MaxBatchSize = i
	}

	if workerPoolSizeString, found := os.LookupEnv("SHORTENER_WORKER_POOL_SIZE"); found {
		u64, err := strconv.ParseUint(workerPoolSizeString, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		config.PendingMessageProcessorParams.WorkerPoolSize = u64
	}

	if maxParallelInsertsString, found := os.LookupEnv("SHORTENER_MAX_PARALLEL_INSERTS"); found {
		u64, err := strconv.ParseUint(maxParallelInsertsString, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		config.PendingMessageProcessorParams.MaxParallelInsertsToQueue = u64
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
