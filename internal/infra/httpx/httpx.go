package httpx

import (
	"errors"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout  = 20 * time.Second
	defaultRetryMax = 2
)

// Transport 把“UA 池 + 代理 + keep-alive 策略 + 有界重试”固化为统一策略。
//
// 设计目标：provider 只负责“定位页面 + 解析 HTML”，不关心网络策略细节。
type Transport struct {
	Base *http.Transport

	ua *uaPool

	// RetryMax 表示最大重试次数（不含首次尝试）。例如 2 表示最多 3 次尝试。
	RetryMax int

	// DisableKeepAlives 决定是否对 Request 设置 Close=true（额外保险）。
	// 真正禁用 keep-alive 依赖 Base.DisableKeepAlives。
	DisableKeepAlives bool
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}
	if t.Base == nil {
		return nil, errors.New("nil base transport")
	}

	// 只对“可重放”的请求做重试：GET/HEAD 且无 body。
	canRetry := (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.Body == nil
	max := t.RetryMax
	if max < 0 {
		max = 0
	}
	if !canRetry {
		max = 0
	}

	var lastErr error
	for attempt := 0; attempt <= max; attempt++ {
		r := cloneRequest(req)
		if r.Header.Get("User-Agent") == "" {
			r.Header.Set("User-Agent", t.ua.random())
		}
		if t.DisableKeepAlives {
			// 额外保险：即使上层误用了其它 Transport，也尽量不复用连接。
			r.Close = true
		}

		resp, err := t.Base.RoundTrip(r)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if req.Context().Err() != nil {
			// ctx 已取消：不再重试，直接返回最后错误（更可解释）。
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func cloneRequest(req *http.Request) *http.Request {
	// Clone 会复制 Header 等，避免在 RoundTripper 内部“污染”调用方的 request。
	return req.Clone(req.Context())
}

// NewMetaClient 构造用于 provider 页面抓取的 HTTP client。
//
// 规则：
// - proxyURL 非空：必须走代理，且禁用 keep-alive（每请求新连接）
// - 内置 UA 池：每个请求随机 UA
// - 有界重试 + 总超时
func NewMetaClient(proxyURL string) (*http.Client, error) {
	return newClient(strings.TrimSpace(proxyURL), false)
}

// NewImageClient 构造用于图片下载的 HTTP client。
//
// 规则：
// - imageProxy=false：图片直连（忽略 proxyURL）
// - imageProxy=true：图片走 proxyURL，且禁用 keep-alive（每请求新连接）
func NewImageClient(proxyURL string, imageProxy bool) (*http.Client, error) {
	if !imageProxy {
		return newClient("", false)
	}
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil, errors.New("image_proxy=true 但 proxy.url 为空")
	}
	return newClient(proxyURL, false)
}

func newClient(proxyURL string, disableKeepAlives bool) (*http.Client, error) {
	base := &http.Transport{
		Proxy:                 nil,
		DisableKeepAlives:     disableKeepAlives,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		base.Proxy = http.ProxyURL(u)
		// proxy 模式强制每请求新连接（代理池轮换依赖该行为）。
		base.DisableKeepAlives = true
		disableKeepAlives = true
	}

	tr := &Transport{
		Base:              base,
		ua:                globalUA,
		RetryMax:          defaultRetryMax,
		DisableKeepAlives: disableKeepAlives,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   defaultTimeout,
	}, nil
}

type uaPool struct {
	mu  sync.Mutex
	rnd *rand.Rand
	uas []string
}

func (p *uaPool) random() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.uas[p.rnd.Intn(len(p.uas))]
}

var globalUA = newUAPool()

func newUAPool() *uaPool {
	// 尽量保持 UA 列表短小但多样；未来可扩充（不对外暴露配置）。
	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	}
	return &uaPool{
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
		uas: uas,
	}
}
