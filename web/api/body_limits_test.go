package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestBodyLimitRunsBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimitRequestBody())
	r.POST("/api/login", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})
	tooLarge := bytes.Repeat([]byte("x"), int(MaxJSONRequestBody)+1)
	request := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(tooLarge))
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(tooLarge))
	request.ContentLength = -1
	recorder = httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unknown-length status = %d, want 413", recorder.Code)
	}
}

func TestRequestBodyLimitPreservesUploadLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimitRequestBody())
	for _, path := range []string{"/api/admin/upload/chunk", "/api/install/upload/chunk", "/api/admin/update/favicon"} {
		r.POST(path, func(c *gin.Context) {
			_, err := io.Copy(io.Discard, c.Request.Body)
			if err != nil {
				c.Status(http.StatusRequestEntityTooLarge)
				return
			}
			c.Status(http.StatusOK)
		})
	}
	for _, tc := range []struct {
		path string
		size int64
	}{
		{"/api/admin/upload/chunk", 5 << 20},
		{"/api/install/upload/chunk", 5 << 20},
		{"/api/admin/update/favicon", 5 << 20},
	} {
		request := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(bytes.Repeat([]byte("x"), int(tc.size))))
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", tc.path, recorder.Code)
		}
	}
}
