package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/provider"
)

func download(ctx context.Context, c *http.Client, u string, configure func(*http.Request)) ([]byte, error) {
	if c == nil {
		return nil, errors.New("image client 为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	if configure != nil {
		configure(req)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func imageRequestConfigurer(reg provider.Registry, providerUsed string, meta domain.MovieMeta) func(*http.Request) {
	p, ok := reg.Get(providerUsed)
	if !ok {
		return nil
	}
	preparer, ok := p.(provider.ImageRequestPreparer)
	if !ok {
		return nil
	}
	return func(req *http.Request) {
		preparer.PrepareImageRequest(req, meta)
	}
}
