package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"slices"
	"strings"
)

// compressWriter — http.ResponseWriter, сжимающий ответ gzip для подходящих Content-Type.
type compressWriter struct {
	http.ResponseWriter
	zw             *gzip.Writer
	needToCompress bool
}

// newCompressWriter оборачивает w в compressWriter.
func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		ResponseWriter: w,
		zw:             gzip.NewWriter(w),
		needToCompress: false,
	}
}

// Write пишет данные, сжимая их, если сжатие включено.
func (c *compressWriter) Write(p []byte) (int, error) {
	if c.needToCompress {
		return c.zw.Write(p)
	} else {
		return c.ResponseWriter.Write(p)
	}
}

// WriteHeader включает сжатие для успешных ответов с подходящим Content-Type
// и выставляет заголовок Content-Encoding.
func (c *compressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 && c.shouldCompress(c.Header().Get("Content-Type")) {
		c.Header().Set("Content-Encoding", "gzip")
		c.needToCompress = true
	}
	c.ResponseWriter.WriteHeader(statusCode)
}

// Close дописывает gzip-поток, если сжатие было включено.
func (c *compressWriter) Close() error {
	if c.needToCompress {
		return c.zw.Close()
	}
	return nil
}

// shouldCompress сообщает, нужно ли сжимать ответ с данным Content-Type.
func (c *compressWriter) shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"application/json",
		"text/html",
	}

	return slices.Contains(compressibleTypes, contentType)
}

// compressReader — io.ReadCloser, распаковывающий gzip-тело запроса.
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

// newCompressReader оборачивает r в compressReader.
func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

// Read читает распакованные данные.
func (c *compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

// Close закрывает gzip-reader и исходное тело запроса.
func (c *compressReader) Close() error {
	if err := c.zr.Close(); err != nil {
		return err
	}
	return c.r.Close()
}

// gzipMiddleware сжимает ответы и распаковывает тела запросов в зависимости от
// заголовков Accept-Encoding и Content-Encoding.
func (router *URLShortenerRouter) gzipMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w

		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			cw := newCompressWriter(w)
			ow = cw
			defer cw.Close()
		}

		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {

			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer cr.Close()
		}

		h.ServeHTTP(ow, r)

	})
}
