// Package service содержит бизнес-логику сокращения URL и сервис аудита.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
)

// auditQueueSize — размер буфера очереди сообщений аудита.
const auditQueueSize = 1024

// auditMessageHandler — получатель сообщений аудита.
type auditMessageHandler interface {
	HandlerName() string
	FetchMessage(message model.AuditServiceMessage) error
	OnServerShutdown() error
}

// AuditService рассылает сообщения аудита всем зарегистрированным получателям (файл, удалённый URL).
// Сообщения помещаются в очередь и обрабатываются фоновым воркером, чтобы не блокировать обработчики запросов.
type AuditService struct {
	handlers []auditMessageHandler

	queue   chan model.AuditServiceMessage
	onError func(err error)
	wg      sync.WaitGroup
	done    chan struct{}
}

// NewAuditService создаёт сервис по конфигурации и запускает фонового воркера.
// onError вызывается для каждой ошибки доставки сообщения получателю (может быть nil).
func NewAuditService(conf config.AuditServiceConfig, onError func(err error)) (*AuditService, error) {
	if len(conf.FilePath) == 0 && len(conf.URL) == 0 {
		return nil, fmt.Errorf("can't initialize audit service because config is empty")
	}

	service := &AuditService{
		handlers: make([]auditMessageHandler, 0),
		queue:    make(chan model.AuditServiceMessage, auditQueueSize),
		onError:  onError,
		done:     make(chan struct{}),
	}

	if len(conf.FilePath) != 0 {
		fileHandler, err := newAuditFileHandler(conf.FilePath)
		if err != nil {
			return nil, fmt.Errorf("can't initialize audit file handler: %w", err)
		}

		service.registerHandler(fileHandler)
	}

	if len(conf.URL) != 0 {
		service.registerHandler(newAuditRemoteHandler(conf.URL))
	}

	service.wg.Add(1)
	go service.worker()

	return service, nil
}

// registerHandler добавляет получателя сообщений.
func (service *AuditService) registerHandler(handler auditMessageHandler) {
	service.handlers = append(service.handlers, handler)
}

// NotifyShorten ставит в очередь аудит-сообщение о сокращении URL.
func (service *AuditService) NotifyShorten(userID uint64, originalURL string) error {
	return service.notify("shorten", userID, originalURL)
}

// NotifyFollow ставит в очередь аудит-сообщение о переходе по короткому URL.
func (service *AuditService) NotifyFollow(userID uint64, originalURL string) error {
	return service.notify("follow", userID, originalURL)
}

// notify формирует сообщение и неблокирующе помещает его в очередь.
func (service *AuditService) notify(action string, userID uint64, originalURL string) error {
	msg := model.AuditServiceMessage{
		Timestamp:   time.Now().Unix(),
		Action:      action,
		UserID:      strconv.FormatUint(userID, 10),
		OriginalURL: originalURL,
	}

	select {
	case <-service.done:
		return fmt.Errorf("audit service is stopped")
	default:
	}

	// Тут возможна гонка, при которой сервису уже послали сигнал о закрытии, он дочитал очередь и воркер вырубился,
	// но notify только потом отправляет в очередь сообщение, из-за этого оно может потеряться. Считаем это нормальным и не критичным.
	select {
	case service.queue <- msg:
		return nil
	default:
		return fmt.Errorf("audit queue is full, %q message dropped", action)
	}
}

// worker последовательно доставляет сообщения из очереди всем получателям.
func (service *AuditService) worker() {
	defer service.wg.Done()

	for {
		select {
		case msg := <-service.queue:
			service.deliver(msg)
		case <-service.done:
			// при остановке доставляем всё, что осталось в очереди
			for {
				select {
				case msg := <-service.queue:
					service.deliver(msg)
				default:
					return
				}
			}
		}
	}
}

// deliver передаёт сообщение всем получателям.
func (service *AuditService) deliver(msg model.AuditServiceMessage) {
	for i := range service.handlers {
		if err := service.handlers[i].FetchMessage(msg); err != nil && service.onError != nil {
			service.onError(fmt.Errorf("error while fetching %q message in handler %q: %w", msg.Action, service.handlers[i].HandlerName(), err))
		}
	}
}

// OnServerShutdown дожидается обработки очереди и запускает graceful-shutdown методы хэндлеров.
func (service *AuditService) OnServerShutdown() error {
	close(service.done)

	service.wg.Wait()

	errs := make([]error, 0)

	for i := range service.handlers {
		if err := service.handlers[i].OnServerShutdown(); err != nil {
			errs = append(errs, fmt.Errorf("handler %q: %w", service.handlers[i].HandlerName(), err))
		}
	}

	return errors.Join(errs...)
}

// auditRemoteHandler отправляет сообщения аудита POST-запросом на внешний URL.
type auditRemoteHandler struct {
	client *resty.Client
}

func newAuditRemoteHandler(URL string) *auditRemoteHandler {
	return &auditRemoteHandler{client: resty.New().SetBaseURL(URL)}
}

func (handler *auditRemoteHandler) HandlerName() string {
	return "Audit remote sender"
}

func (handler *auditRemoteHandler) FetchMessage(message model.AuditServiceMessage) error {
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := handler.client.R().SetContext(reqCtx).SetBody(message).Post("")
	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("audit receiver returned status %d", resp.StatusCode())
	}

	return nil
}

func (handler *auditRemoteHandler) OnServerShutdown() error {
	return nil
}

// auditFileHandler дописывает сообщения аудита в файл в формате JSON.
type auditFileHandler struct {
	fileMtx  sync.Mutex
	fileDesc *os.File
}

func newAuditFileHandler(filePath string) (*auditFileHandler, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &auditFileHandler{fileDesc: f}, nil
}

func (handler *auditFileHandler) HandlerName() string {
	return "Audit file sender"
}

func (handler *auditFileHandler) FetchMessage(message model.AuditServiceMessage) error {
	handler.fileMtx.Lock()
	defer handler.fileMtx.Unlock()

	return json.NewEncoder(handler.fileDesc).Encode(message)
}

func (handler *auditFileHandler) OnServerShutdown() error {
	handler.fileMtx.Lock()
	defer handler.fileMtx.Unlock()

	return handler.fileDesc.Close()
}
