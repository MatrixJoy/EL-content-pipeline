package source

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientReadsGzipByMagicWithoutContentEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		writer := gzip.NewWriter(w)
		_, _ = writer.Write([]byte(`<urlset><url><loc>https://example.com/a/1.html</loc></url></urlset>`))
		_ = writer.Close()
	}))
	defer server.Close()
	c := NewVOA(server.URL, "test", time.Millisecond)
	body, _, err := c.get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `<urlset><url><loc>https://example.com/a/1.html</loc></url></urlset>` {
		t.Fatalf("body = %q", body)
	}
}
