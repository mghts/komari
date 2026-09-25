package client

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/komari-monitor/komari/web/api"
)

func TestCompressedReportBodyHasExpandedSizeLimit(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("x"), int(api.MaxJSONRequestBody)+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/clients/v2/rpc", bytes.NewReader(compressed.Bytes()))
	request.Header.Set("Content-Encoding", "gzip")
	if _, err := readMaybeCompressedBody(request); err == nil {
		t.Fatal("gzip body exceeding the expanded size limit was accepted")
	}
}
