// Package service содержит бизнес-логику сокращения URL и сервис аудита.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
)

// auditMessageHandler — получатель сообщений аудита.
type auditMessageHandler interface {
	HandlerName() string
	FetchMessage(message model.AuditServiceMessage) error
}

// AuditService рассылает сообщения аудита всем зарегистрированным получателям (файл, удалённый URL).
type AuditService struct {
	handlers []auditMessageHandler
}

// NewAuditService создаёт сервис по конфигурации. Возвращает ошибку, если не задан ни файл, ни URL.
func NewAuditService(conf config.AuditServiceConfig) (*AuditService, error) {
	if len(conf.FilePath) == 0 && len(conf.URL) == 0 {
		return nil, fmt.Errorf("can't initialize audit service because config is empty")
	}

	service := &AuditService{handlers: make([]auditMessageHandler, 0)}

	if len(conf.FilePath) != 0 {
		service.registerHandler(newAuditFileHandler(conf.FilePath))
	}

	if len(conf.URL) != 0 {
		service.registerHandler(newAuditRemoteHandler(conf.URL))
	}

	return service, nil
}

// registerHandler добавляет получателя сообщений.
func (service *AuditService) registerHandler(handler auditMessageHandler) {
	service.handlers = append(service.handlers, handler)
}

// NotifyShorten отправляет аудит-сообщение о сокращении URL и возвращает ошибки получателей.
func (service *AuditService) NotifyShorten(userID uint64, originalURL string) []error {
	return service.notify("shorten", userID, originalURL)
}

// NotifyFollow отправляет аудит-сообщение о переходе по короткому URL и возвращает ошибки получателей.
func (service *AuditService) NotifyFollow(userID uint64, originalURL string) []error {
	return service.notify("follow", userID, originalURL)
}

// notify формирует сообщение и передаёт его всем получателям, собирая ошибки.
func (service *AuditService) notify(action string, userID uint64, originalURL string) []error {
	msg := model.AuditServiceMessage{
		Timestamp:   time.Now().Unix(),
		Action:      action,
		UserID:      userID,
		OriginalURL: originalURL,
	}

	errs := make([]error, 0)

	for i := range service.handlers {
		if err := service.handlers[i].FetchMessage(msg); err != nil {
			errs = append(errs, fmt.Errorf("error while fetching %q message in handler %q. Error: %w", action, service.handlers[i].HandlerName(), err))
		}
	}

	return errs
}

// auditRemoteHandler отправляет сообщения аудита POST-запросом на внешний URL.
type auditRemoteHandler struct {
	url string
}

func newAuditRemoteHandler(URL string) *auditRemoteHandler {
	return &auditRemoteHandler{url: URL}
}

func (handler *auditRemoteHandler) HandlerName() string {
	return "Audit remote sender"
}

func (handler *auditRemoteHandler) FetchMessage(message model.AuditServiceMessage) error {
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := resty.New().R().SetContext(reqCtx).SetBody(message).Post(handler.url)

	return err
}

// auditFileHandler дописывает сообщения аудита в файл в формате JSON.
type auditFileHandler struct {
	filePath string
	fileMtx  sync.Mutex
}

func newAuditFileHandler(filePath string) *auditFileHandler {
	return &auditFileHandler{filePath: filePath}
}

func (handler *auditFileHandler) HandlerName() string {
	return "Audit file sender"
}

func (handler *auditFileHandler) FetchMessage(message model.AuditServiceMessage) error {
	handler.fileMtx.Lock()
	defer handler.fileMtx.Unlock()

	f, err := os.OpenFile(handler.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(message); err != nil {
		return err
	}

	return nil
}
