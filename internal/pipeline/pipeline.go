package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/oldj/english-learning/content-pipeline/internal/classify"
	"github.com/oldj/english-learning/content-pipeline/internal/domain"
	"github.com/oldj/english-learning/content-pipeline/internal/source"
	"github.com/oldj/english-learning/content-pipeline/internal/store"
)

type Runner struct {
	Source     source.Connector
	Classifier classify.Classifier
	State      *store.StateDB
	Objects    *store.Objects
	Logger     *slog.Logger
}

func (r *Runner) Run(ctx context.Context, limit int) (int, error) {
	items, err := r.Source.Discover(ctx)
	if err != nil {
		return 0, err
	}
	r.Logger.Info("discovery complete", "source", r.Source.ID(), "articles", len(items))
	urls := make([]string, 0, len(items))
	for _, item := range items {
		urls = append(urls, item.URL)
	}
	return r.RunURLs(ctx, urls, limit)
}

func (r *Runner) RunURLs(ctx context.Context, urls []string, limit int) (int, error) {
	processed := 0
	for _, u := range urls {
		stateKey := fmt.Sprintf("%s|extract-v%d|%s", r.Source.ID(), r.Source.ExtractionVersion(), u)
		if r.State.Done(stateKey) {
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
			_ = r.State.Set(stateKey, status, err.Error())
			r.Logger.Warn("candidate rejected", "url", u, "reason", err)
			continue
		}
		_ = r.State.Set(stateKey, "complete", "")
		r.Logger.Info("candidate stored", "url", u)
	}
	return processed, nil
}
func (r *Runner) one(ctx context.Context, u string) error {
	c, err := r.Source.FetchCandidate(ctx, u)
	if err != nil {
		return err
	}
	prefix := path.Join("candidates", r.Source.ID(), c.Article.ContentID)
	audioKey := path.Join(prefix, "audio.mp3")
	audio, mediaType, err := r.Objects.Get(ctx, audioKey)
	if err != nil {
		audio, mediaType, err = r.Source.Download(ctx, c.AudioURL)
		if err != nil {
			return fmt.Errorf("download audio: %w", err)
		}
	}
	if !strings.Contains(strings.ToLower(mediaType), "audio") && !strings.Contains(strings.ToLower(mediaType), "mpeg") {
		return fmt.Errorf("unexpected audio type %q", mediaType)
	}
	articleJSON, _ := json.MarshalIndent(c.Article, "", "  ")
	objects := map[string]domain.Object{}
	for name, item := range map[string]struct {
		key, kind string
		data      []byte
	}{"article": {path.Join(prefix, "article.json"), "application/json", articleJSON}, "source": {path.Join(prefix, "source.html"), "text/html", c.HTML}, "audio": {audioKey, "audio/mpeg", audio}} {
		obj, e := r.Objects.Put(ctx, item.key, item.kind, item.data)
		if e != nil {
			return e
		}
		objects[name] = obj
	}
	classification := r.Classifier.Classify(c.Article)
	manifest := domain.Manifest{SchemaVersion: 2, ContentID: c.Article.ContentID, Revision: 1, Status: "candidate", Source: r.Source.ID(), SourceURL: u, CapturedAt: time.Now().UTC(), Classification: classification, Objects: objects, Attribution: r.Source.Attribution()}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	_, err = r.Objects.Put(ctx, path.Join(prefix, "manifest.json"), "application/json", data)
	return err
}
