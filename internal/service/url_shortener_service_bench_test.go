package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func benchSetErr(b *testing.B, err error) {
	b.Helper()
	if err != nil && !errors.Is(err, repository.ErrFullURLCollision) {
		b.Fatal(err)
	}
}

type atomicCounter struct{ v atomic.Uint64 }

func (c *atomicCounter) next() uint64 { return c.v.Add(1) }

func newBenchService(b *testing.B) *URLShortenerService {
	b.Helper()

	l := zap.NewNop()

	params := config.PendingMessageProcessorConfig{
		TickTime:                  1,
		BufferSize:                1024,
		MaxBatchSize:              512,
		WorkerPoolSize:            4,
		MaxParallelInsertsToQueue: 64,
	}

	storage, err := repository.NewRuntimeStorage("", &params, l)
	require.NoError(b, err)

	serv, err := NewURLShortenerService("http://localhost:8080", storage)
	require.NoError(b, err)

	b.Cleanup(func() { _ = serv.OnServerShutdown() })

	return serv
}

func fillService(b *testing.B, serv *URLShortenerService, userID uint64, n int) []string {
	b.Helper()

	ctx := context.Background()
	keys := make([]string, 0, n)
	for i := range n {
		resp, err := serv.SetFullURL(ctx, model.RequestURLData{
			OriginalURL: "https://example.com/fill/" + strconv.FormatUint(userID, 10) + "/" + strconv.Itoa(i),
			UserID:      userID,
		})
		if err != nil && !errors.Is(err, repository.ErrFullURLCollision) {
			b.Fatal(err)
		}
		keys = append(keys, resp.ShortURL[len(serv.shortURLPrefix):])
	}
	return keys
}

func BenchmarkGetFullURL(b *testing.B) {
	serv := newBenchService(b)
	keys := fillService(b, serv, 1, 10000)
	ctx := context.Background()

	b.ResetTimer()

	b.Run("sequential", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; b.Loop(); i++ {
			if _, err := serv.GetFullURL(ctx, keys[i%len(keys)]); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if _, err := serv.GetFullURL(ctx, keys[i%len(keys)]); err != nil {
					b.Error(err)
					return
				}
				i++
			}
		})
	})
}

func BenchmarkGetUserURLs(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("urls=%d", n), func(b *testing.B) {
			serv := newBenchService(b)
			fillService(b, serv, 1, n)
			// noise from another user
			fillService(b, serv, 2, n)
			ctx := context.Background()
			b.ResetTimer()

			b.ReportAllocs()
			for b.Loop() {
				res, err := serv.GetUserURLs(ctx, 1)
				if err != nil {
					b.Fatal(err)
				}
				if len(res) != n {
					b.Fatalf("expected %d urls, got %d", n, len(res))
				}
			}
		})
	}
}

func BenchmarkDeleteUserURLs(b *testing.B) {
	for _, n := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("batch=%d", n), func(b *testing.B) {
			serv := newBenchService(b)
			keys := fillService(b, serv, 1, n)
			b.ResetTimer()

			b.ReportAllocs()
			for b.Loop() {
				serv.DeleteUserURLs(1, keys)
			}
		})
	}
}

func BenchmarkSetFullURL(b *testing.B) {
	b.Run("unique", func(b *testing.B) {
		serv := newBenchService(b)
		ctx := context.Background()
		b.ResetTimer()

		b.ReportAllocs()
		for i := 0; b.Loop(); i++ {
			_, err := serv.SetFullURL(ctx, model.RequestURLData{
				OriginalURL: "https://example.com/" + strconv.Itoa(i),
				UserID:      1,
			})
			benchSetErr(b, err)
		}
	})

	b.Run("parallel_unique", func(b *testing.B) {
		serv := newBenchService(b)
		ctx := context.Background()
		var counter atomicCounter
		b.ResetTimer()

		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := serv.SetFullURL(ctx, model.RequestURLData{
					OriginalURL: "https://example.com/p/" + strconv.FormatUint(counter.next(), 10),
					UserID:      1,
				})
				if err != nil && !errors.Is(err, repository.ErrFullURLCollision) {
					b.Error(err)
					return
				}
			}
		})
	})
}

func BenchmarkSetFullURLBatch(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("batch=%d", n), func(b *testing.B) {
			serv := newBenchService(b)
			ctx := context.Background()
			var counter atomicCounter

			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				batch := make([]model.RequestURLData, n)
				for i := range batch {
					id := counter.next()
					batch[i] = model.RequestURLData{
						OriginalURL:   "https://example.com/batch/" + strconv.FormatUint(id, 10),
						CorrelationID: strconv.FormatUint(id, 10),
						UserID:        1,
					}
				}
				b.StartTimer()

				_, err := serv.SetFullURLBatch(ctx, batch)
				benchSetErr(b, err)
			}
		})
	}
}
