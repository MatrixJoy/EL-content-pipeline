package source

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

type Client struct {
	http         *http.Client
	userAgent    string
	delay        time.Duration
	mu           sync.Mutex
	last         time.Time
	sitemap      string
	maxArticleID int
}

func NewVOA(sitemap, userAgent string, delay time.Duration, maxArticleID int) *Client {
	return &Client{http: &http.Client{Timeout: 2 * time.Minute}, sitemap: sitemap, userAgent: userAgent, delay: delay, maxArticleID: maxArticleID}
}

func (c *Client) ID() string          { return "voa-learning-english" }
func (c *Client) Attribution() string { return "VOA Learning English" }

func (c *Client) get(ctx context.Context, target string) ([]byte, string, error) {
	c.mu.Lock()
	wait := time.Until(c.last.Add(c.delay))
	if wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			c.mu.Unlock()
			return nil, "", ctx.Err()
		case <-timer.C:
		}
	}
	c.last = time.Now()
	c.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", target, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err == nil && len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		reader, openErr := gzip.NewReader(bytes.NewReader(body))
		if openErr != nil {
			return nil, "", fmt.Errorf("open gzip %s: %w", target, openErr)
		}
		body, err = io.ReadAll(reader)
		closeErr := reader.Close()
		if err == nil {
			err = closeErr
		}
	}
	return body, resp.Header.Get("Content-Type"), err
}

type sitemap struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
	URLs []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
}

func (c *Client) Discover(ctx context.Context) ([]domain.Item, error) {
	seen, articles := map[string]bool{}, map[string]domain.Item{}
	var walk func(string) error
	walk = func(target string) error {
		if seen[target] {
			return nil
		}
		seen[target] = true
		body, _, err := c.get(ctx, target)
		if err != nil {
			return err
		}
		var sm sitemap
		if err = xml.Unmarshal(body, &sm); err != nil {
			return fmt.Errorf("decode sitemap %s: %w", target, err)
		}
		for _, child := range sm.Sitemaps {
			low := strings.ToLower(child.Loc)
			if strings.Contains(low, "video") {
				continue
			}
			if err := walk(child.Loc); err != nil {
				return err
			}
		}
		for _, item := range sm.URLs {
			u, err := url.Parse(item.Loc)
			if err == nil && u.Host == "learningenglish.voanews.com" && strings.HasPrefix(u.Path, "/a/") && strings.HasSuffix(u.Path, ".html") && c.withinArticleRange(item.Loc) {
				modified, _ := time.Parse(time.RFC3339, item.LastMod)
				if modified.IsZero() {
					modified, _ = time.Parse("2006-01-02", item.LastMod)
				}
				articles[item.Loc] = domain.Item{URL: item.Loc, LastModified: modified}
			}
		}
		return nil
	}
	if err := walk(c.sitemap); err != nil {
		return nil, err
	}
	out := make([]domain.Item, 0, len(articles))
	for _, item := range articles {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastModified.After(out[j].LastModified) })
	return out, nil
}

func (c *Client) withinArticleRange(rawURL string) bool {
	if c.maxArticleID <= 0 {
		return true
	}
	match := idPattern.FindStringSubmatch(rawURL)
	if len(match) < 2 {
		return false
	}
	id, err := strconv.Atoi(match[1])
	return err == nil && id <= c.maxArticleID
}

var idPattern = regexp.MustCompile(`(\d+)\.html$`)

func (c *Client) FetchCandidate(ctx context.Context, pageURL string) (domain.Candidate, error) {
	body, _, err := c.get(ctx, pageURL)
	if err != nil {
		return domain.Candidate{}, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return domain.Candidate{}, err
	}
	audio := findAudio(doc, pageURL)
	if audio == "" {
		return domain.Candidate{}, fmt.Errorf("no audio")
	}
	title := clean(doc.Find("h1").First().Text())
	if title == "" {
		return domain.Candidate{}, fmt.Errorf("no title")
	}
	idMatch := idPattern.FindStringSubmatch(pageURL)
	if len(idMatch) < 2 {
		return domain.Candidate{}, fmt.Errorf("no source id")
	}
	paragraphs := make([]domain.Paragraph, 0)
	doc.Find(".wsw p, .article-body p, .article__body p").Each(func(_ int, s *goquery.Selection) {
		t := clean(s.Text())
		if len(t) >= 20 && !isBoilerplate(t) {
			paragraphs = append(paragraphs, domain.Paragraph{Index: len(paragraphs), Text: t})
		}
	})
	if len(paragraphs) == 0 {
		return domain.Candidate{}, fmt.Errorf("no learning text")
	}
	series := clean(doc.Find(".category, .page-header__category").First().Text())
	keywords := doc.Find(`meta[name="keywords"]`).AttrOr("content", "")
	wordCount := 0
	for _, paragraph := range paragraphs {
		wordCount += len(strings.Fields(paragraph.Text))
	}
	article := domain.Article{SchemaVersion: 2, ContentID: "voa-" + idMatch[1], SourceURL: pageURL, Title: title, Description: doc.Find(`meta[name="description"]`).AttrOr("content", ""), Series: series, PublishedAt: published(doc), Level: level(series + " " + keywords), Topics: topics(series, keywords), Language: "en", WordCount: wordCount, Paragraphs: paragraphs}
	return domain.Candidate{Article: article, HTML: body, AudioURL: audio, AudioType: "audio/mpeg"}, nil
}

func isBoilerplate(text string) bool {
	value := strings.ToLower(strings.TrimSpace(text))
	for _, phrase := range []string{"no media source currently available", "the code has been copied to your clipboard", "write to us in the comments section", "share your thoughts in the comments section"} {
		if strings.HasPrefix(value, phrase) {
			return true
		}
	}
	return false
}

func (c *Client) Download(ctx context.Context, target string) ([]byte, string, error) {
	return c.get(ctx, target)
}

func findAudio(doc *goquery.Document, base string) string {
	var choices []string
	doc.Find(`a[href], audio[src], source[src]`).Each(func(_ int, s *goquery.Selection) {
		v, ok := s.Attr("href")
		if !ok {
			v, _ = s.Attr("src")
		}
		low := strings.ToLower(v)
		if strings.Contains(low, ".mp3") {
			if u, err := url.Parse(v); err == nil {
				b, _ := url.Parse(base)
				choices = append(choices, b.ResolveReference(u).String())
			}
		}
	})
	sort.SliceStable(choices, func(i, j int) bool { return bitrate(choices[i]) > bitrate(choices[j]) })
	if len(choices) > 0 {
		return choices[0]
	}
	return findJSONAudio(doc)
}
func findJSONAudio(doc *goquery.Document) string {
	var result string
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var v any
		if json.Unmarshal([]byte(s.Text()), &v) != nil {
			return true
		}
		result = walkJSON(v)
		return result == ""
	})
	return result
}
func walkJSON(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			if k == "contentUrl" || k == "url" {
				if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), ".mp3") {
					return s
				}
			}
			if s := walkJSON(v); s != "" {
				return s
			}
		}
	case []any:
		for _, v := range x {
			if s := walkJSON(v); s != "" {
				return s
			}
		}
	}
	return ""
}
func bitrate(s string) int {
	if strings.Contains(s, "128") {
		return 128
	}
	if strings.Contains(s, "64") {
		return 64
	}
	return 1
}
func clean(s string) string { return strings.Join(strings.Fields(s), " ") }
func published(doc *goquery.Document) string {
	for _, sel := range []string{`meta[property="article:published_time"]`, `meta[name="date"]`, `time[datetime]`} {
		s := doc.Find(sel).First()
		if v, ok := s.Attr("content"); ok {
			return v
		}
		if v, ok := s.Attr("datetime"); ok {
			return v
		}
	}
	return ""
}
func level(s string) string {
	x := strings.ToLower(s)
	switch {
	case strings.Contains(x, "advanced"):
		return "advanced"
	case strings.Contains(x, "intermediate"):
		return "intermediate"
	case strings.Contains(x, "beginning"), strings.Contains(x, "let's learn english"):
		return "beginning"
	}
	return "unclassified"
}
func topics(series, keywords string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range strings.Split(series+","+keywords, ",") {
		v = clean(v)
		if v != "" && len(v) < 60 && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}
