package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/model"
	"go.uber.org/zap"
)

type URLStorage interface {
	GetFullURL(ctx context.Context, shortURL string) (string, error)
	GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error)
	DeleteUserURLs(deleteUserURLs DeleteUserURLsDTO)
	SetURL(ctx context.Context, URLRecord model.URLRecord) (model.ResponcePostURLData, error)
	SetURLBatch(ctx context.Context, URLRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedURLRecords, error)
	CreateNewUser(ctx context.Context) (uint64, error)
	OnServerShutdown() error
	Ping(ctx context.Context) error
}

type UnacceptedURLRecords []model.URLRecord

type DeleteUserURLsDTO struct {
	UserID    uint64
	ShortURLs []string
}

var (
	ErrShortURLCollision = errors.New("storage already have this short_url")
	ErrNotFound          = errors.New("can't find record")
	ErrDeleted           = errors.New("record was deleted")
	ErrDeleteProhibited  = errors.New("delete URLs by this user prohibited")
	ErrFullURLCollision  = errors.New("storage already have this URL(s)")
)

type PendingMessageProcessor[T any] struct {
	messageQueue   chan T
	tickTime       time.Duration
	tickFunc       func([]T) error
	stopSig        chan struct{}
	maxBatchSize   int
	workerPoolSize uint64
	insertSem      chan struct{}
	logger         *zap.Logger
	wg             sync.WaitGroup
}

func NewPendingMessageProcessor[T any](tick time.Duration, bufferSize uint64, maxBatchSize int, workerPoolSize uint64, insertSemSize uint64,
	tickFunc func(messages []T) error, logger *zap.Logger) *PendingMessageProcessor[T] {

	return &PendingMessageProcessor[T]{
		messageQueue:   make(chan T, bufferSize),
		tickTime:       tick,
		tickFunc:       tickFunc,
		stopSig:        make(chan struct{}),
		maxBatchSize:   maxBatchSize,
		workerPoolSize: workerPoolSize,
		insertSem:      make(chan struct{}, insertSemSize),
		logger:         logger,
	}
}

func (processor *PendingMessageProcessor[T]) Run() {
	for range processor.workerPoolSize {
		processor.wg.Go(func() {
			ticker := time.NewTicker(processor.tickTime)

			var messages []T

			processMessages := func() {
				if len(messages) == 0 {
					return
				}
				// сохраним все пришедшие сообщения одновременно
				err := processor.tickFunc(messages)
				if err != nil {
					processor.logger.Debug("cannot process messages", zap.Error(err))
					return
				}
				// сотрём успешно отосланные сообщения
				messages = nil
			}

			for {
				select {
				case msg := <-processor.messageQueue:
					messages = append(messages, msg)
					if len(messages) >= processor.maxBatchSize {
						processMessages()
					}
				case <-ticker.C:
					processMessages()
				case <-processor.stopSig:
					ticker.Stop()
					processMessages()
					return
				}
			}
		})
	}
}

func (processor *PendingMessageProcessor[T]) Shutdown() {
	processor.stopSig <- struct{}{}
	processor.wg.Wait()
}

func (processor *PendingMessageProcessor[T]) InsertToQueue(message T) {
	processor.insertSem <- struct{}{}

	go func() {
		processor.messageQueue <- message
		<-processor.insertSem
	}()
}
