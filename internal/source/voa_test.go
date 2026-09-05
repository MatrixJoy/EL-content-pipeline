package source

import (
	"compress/gzip"
	"context"
	"fmt"
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
	c := NewVOA(server.URL, "test", time.Millisecond, 0)
	body, _, err := c.get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `<urlset><url><loc>https://example.com/a/1.html</loc></url></urlset>` {
		t.Fatalf("body = %q", body)
	}
}

func TestFetchCandidateExtractsFeaturedWordsAndRemovesFooter(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/audio.mp3" {
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("audio"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><head><meta name="description" content="A lesson"></head><body>
<h1>A useful lesson</h1><div class="wsw">
<p>This is the article paragraph and it contains enough words to be useful.</p>
<h2><strong>Words in This Story</strong></h2>
<p><strong>pest</strong> –n. an animal or insect that causes problems</p>
<p><strong>hang out</strong> - <em>phr v.</em> to spend time together</p>
<p><strong>kitche</strong>n - n. the room where food is made</p>
<p>We want to hear from you in the comments section.</p>
</div><audio src="%s/audio.mp3"></audio></body></html>`, server.URL)
	}))
	defer server.Close()

	client := NewVOA(server.URL, "test", 0, 0)
	candidate, err := client.FetchCandidate(context.Background(), server.URL+"/6685283.html")
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Article.SchemaVersion != 3 {
		t.Fatalf("schema version = %d", candidate.Article.SchemaVersion)
	}
	if len(candidate.Article.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v", candidate.Article.Paragraphs)
	}
	if len(candidate.Article.FeaturedWords) != 3 {
		t.Fatalf("featured words = %#v", candidate.Article.FeaturedWords)
	}
	if got := candidate.Article.FeaturedWords[0]; got.Word != "pest" || got.PartOfSpeech != "noun" || got.Definition != "an animal or insect that causes problems" {
		t.Fatalf("first featured word = %#v", got)
	}
	if got := candidate.Article.FeaturedWords[1]; got.PartOfSpeech != "phrasal verb" || got.Definition != "to spend time together" {
		t.Fatalf("second featured word = %#v", got)
	}
	if got := candidate.Article.FeaturedWords[2]; got.Word != "kitchen" || got.PartOfSpeech != "noun" || got.Definition != "the room where food is made" {
		t.Fatalf("malformed source word = %#v", got)
	}
}
