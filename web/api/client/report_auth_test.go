package client

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAgentCannotReportAsAnotherClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/clients/report", bytes.NewBufferString(`{"uuid":"another-client"}`))
	c.Set("client_uuid", "authenticated-client")
	UploadReport(c)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-node report status = %d, want 403", recorder.Code)
	}
}
