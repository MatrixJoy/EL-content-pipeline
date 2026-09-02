package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/oldj/voa-content-pipeline/internal/domain"
	"github.com/oldj/voa-content-pipeline/internal/source"
	"github.com/oldj/voa-content-pipeline/internal/store"
)

type Runner struct {
	Source  *source.Client
	State   *store.StateDB
	Objects *store.Objects
	Logger  *slog.Logger
}

func (r *Runner) Run(ctx context.Context, sitemap string, limit int) error {
	urls, err := r.Source.Discover(ctx, sitemap)
	if err != nil {
		return err
	}
	r.Logger.Info("discovery complete", "articles", len(urls))
	return r.RunURLs(ctx, urls, limit)
}

func (r *Runner) RunURLs(ctx context.Context, urls []string, limit int) error {
	processed := 0
	for _, u := range urls {
		if r.State.Done(u) {
			continue
		}
		if limit > 0 && processed >= limit {
			break
		}
		processed++
		if err := r.one(ctx, u); err != nil {
			status := "failed"
			if strings.Contains(err.Error(), "no audio") || strings.Contains(err.Error(), "no learning text") {
				status = "skipped"
			}
			_ = r.State.Set(u, status, err.Error())
			r.Logger.Warn("candidate rejected", "url", u, "reason", err)
			continue
		}
		_ = r.State.Set(u, "complete", "")
		r.Logger.Info("candidate stored", "url", u)
	}
	return nil
}
func (r *Runner) one(ctx context.Context, u string) error {
	c, err := r.Source.FetchCandidate(ctx, u)
	if err != nil {
		return err
	}
	audio, mediaType, err := r.Source.Download(ctx, c.AudioURL)
	if err != nil {
		return fmt.Errorf("download audio: %w", err)
	}
	if !strings.Contains(strings.ToLower(mediaType), "audio") && !strings.Contains(strings.ToLower(mediaType), "mpeg") {
		return fmt.Errorf("unexpected audio type %q", mediaType)
	}
	prefix := path.Join("candidates", "voa", c.Article.ContentID)
	articleJSON, _ := json.MarshalIndent(c.Article, "", "  ")
	objects := map[string]domain.Object{}
	for name, item := range map[string]struct {
		key, kind string
		data      []byte
	}{"article": {path.Join(prefix, "article.json"), "application/json", articleJSON}, "source": {path.Join(prefix, "source.html"), "text/html", c.HTML}, "audio": {path.Join(prefix, "audio.mp3"), "audio/mpeg", audio}} {
		obj, e := r.Objects.Put(ctx, item.key, item.kind, item.data)
		if e != nil {
			return e
		}
		objects[name] = obj
	}
	manifest := domain.Manifest{SchemaVersion: 1, ContentID: c.Article.ContentID, Revision: 1, Status: "candidate", Source: "voa-learning-english", SourceURL: u, CapturedAt: time.Now().UTC(), Classification: map[string]any{"level": c.Article.Level, "series": c.Article.Series, "topics": c.Article.Topics, "media": "audio"}, Objects: objects, Attribution: "VOA Learning English"}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	_, err = r.Objects.Put(ctx, path.Join(prefix, "manifest.json"), "application/json", data)
	return err
}
