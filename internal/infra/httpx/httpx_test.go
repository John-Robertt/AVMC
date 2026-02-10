package httpx

import "testing"

func TestNewMetaClient_ProxyDisablesKeepAlive(t *testing.T) {
	c, err := NewMetaClient("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	tr, ok := c.Transport.(*Transport)
	if !ok {
		t.Fatalf("期望 *Transport，实际 %T", c.Transport)
	}
	if tr.Base.Proxy == nil {
		t.Fatalf("期望启用代理，但 Proxy=nil")
	}
	if !tr.Base.DisableKeepAlives {
		t.Fatalf("期望禁用 keep-alive，但 Base.DisableKeepAlives=false")
	}
	if !tr.DisableKeepAlives {
		t.Fatalf("期望设置 Request.Close=true 的额外保险，但 DisableKeepAlives=false")
	}
}

func TestNewMetaClient_NoProxyKeepsDefault(t *testing.T) {
	c, err := NewMetaClient("")
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	tr, ok := c.Transport.(*Transport)
	if !ok {
		t.Fatalf("期望 *Transport，实际 %T", c.Transport)
	}
	if tr.Base.Proxy != nil {
		t.Fatalf("不期望启用代理，但 Proxy!=nil")
	}
	if tr.Base.DisableKeepAlives {
		t.Fatalf("不期望禁用 keep-alive，但 Base.DisableKeepAlives=true")
	}
}

func TestNewImageClient_ImageProxySwitch(t *testing.T) {
	c1, err := NewImageClient("http://127.0.0.1:8080", false)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	tr1 := c1.Transport.(*Transport)
	if tr1.Base.Proxy != nil {
		t.Fatalf("image_proxy=false 时不应走代理")
	}
	if tr1.Base.DisableKeepAlives {
		t.Fatalf("image_proxy=false 时不应禁用 keep-alive")
	}

	c2, err := NewImageClient("http://127.0.0.1:8080", true)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	tr2 := c2.Transport.(*Transport)
	if tr2.Base.Proxy == nil {
		t.Fatalf("image_proxy=true 时应走代理")
	}
	if !tr2.Base.DisableKeepAlives {
		t.Fatalf("image_proxy=true 时应禁用 keep-alive")
	}
}

func TestNewMetaClient_InvalidProxyURL(t *testing.T) {
	_, err := NewMetaClient("http://[::1")
	if err == nil {
		t.Fatalf("期望错误，但得到 nil")
	}
}
