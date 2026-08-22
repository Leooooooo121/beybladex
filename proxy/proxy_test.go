package proxy

import (
	"testing"
	"time"
)

func TestProxyManager_RoundRobin(t *testing.T) {
	proxies := []string{
		"http://127.0.0.1:8001",
		"http://127.0.0.1:8002",
		"http://127.0.0.1:8003",
	}

	pm := NewProxyManager(proxies, 1*time.Second)
	if pm.TotalCount() != 3 {
		t.Fatalf("預期 3 個 Proxy，得到 %d", pm.TotalCount())
	}

	// 測試連續 6 次輪詢
	p1, _ := pm.GetNext()
	p2, _ := pm.GetNext()
	p3, _ := pm.GetNext()
	p4, _ := pm.GetNext()

	if p1 == p2 || p2 == p3 {
		t.Errorf("Round Robin 取得重複 Proxy: p1=%s, p2=%s, p3=%s", p1, p2, p3)
	}
	if p4 != p1 {
		t.Errorf("Round Robin 第四次預期回到 p1 (%s)，但得到 %s", p1, p4)
	}
}

func TestProxyManager_CooldownAndBan(t *testing.T) {
	proxies := []string{
		"http://127.0.0.1:8001",
		"http://127.0.0.1:8002",
	}

	pm := NewProxyManager(proxies, 200*time.Millisecond)

	// 標記 8001 失敗
	pm.ReportFailure("http://127.0.0.1:8001", 429)

	if pm.ActiveCount() != 1 {
		t.Errorf("預期 1 個可用 Proxy，得到 %d", pm.ActiveCount())
	}

	// 取得 Proxy 應只有 8002
	for i := 0; i < 3; i++ {
		p, err := pm.GetNext()
		if err != nil {
			t.Fatalf("GetNext 失敗: %v", err)
		}
		if p != "http://127.0.0.1:8002" {
			t.Errorf("冷卻期間應避開 8001，但取得: %s", p)
		}
	}

	// 等待冷卻結束 (250ms)
	time.Sleep(250 * time.Millisecond)

	if pm.ActiveCount() != 2 {
		t.Errorf("冷卻結束後預期 2 個可用 Proxy，得到 %d", pm.ActiveCount())
	}
}

func TestProxyManager_DirectConnection(t *testing.T) {
	pm := NewProxyManager([]string{}, 1*time.Second)
	p, err := pm.GetNext()
	if err != nil {
		t.Fatalf("無 Proxy 時不應報錯: %v", err)
	}
	if p != "" {
		t.Errorf("無 Proxy 時應回傳空字串，得到: %s", p)
	}
}
