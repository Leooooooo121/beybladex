package proxy

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ProxyStatus 紀錄單一 Proxy 的運作狀態
type ProxyStatus struct {
	URL       string
	Banned    bool
	BannedAt  time.Time
	Cooldown  time.Duration
	FailCount int
	TotalUses uint64
}

// ProxyManager 線程安全代理池管理器
type ProxyManager struct {
	mu           sync.RWMutex
	proxies      []*ProxyStatus
	currentIndex uint64
	cooldown     time.Duration
}

// NewProxyManager 建立代理池管理器
func NewProxyManager(proxyURLs []string, cooldown time.Duration) *ProxyManager {
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}

	pm := &ProxyManager{
		proxies:  make([]*ProxyStatus, 0, len(proxyURLs)),
		cooldown: cooldown,
	}

	for _, p := range proxyURLs {
		if p != "" {
			pm.proxies = append(pm.proxies, &ProxyStatus{
				URL:      p,
				Cooldown: cooldown,
			})
		}
	}

	return pm
}

// AddProxy 動態新增 Proxy
func (pm *ProxyManager) AddProxy(proxyURL string) {
	if proxyURL == "" {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, p := range pm.proxies {
		if p.URL == proxyURL {
			return
		}
	}

	pm.proxies = append(pm.proxies, &ProxyStatus{
		URL:      proxyURL,
		Cooldown: pm.cooldown,
	})
}

// GetNext 透過 Round-Robin 取得下一個可用的 Proxy，若無設定或無可用 Proxy 則回傳空字串 (Direct 連線)
func (pm *ProxyManager) GetNext() (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	n := len(pm.proxies)
	if n == 0 {
		return "", nil // 直接連線
	}

	now := time.Now()
	// 遍歷一輪尋找可用的 Proxy
	for i := 0; i < n; i++ {
		idx := atomic.AddUint64(&pm.currentIndex, 1) % uint64(n)
		ps := pm.proxies[idx]

		// 檢查是否處於冷卻期
		if ps.Banned {
			if now.Sub(ps.BannedAt) >= ps.Cooldown {
				// 冷卻已過，自動解封
				ps.Banned = false
				ps.FailCount = 0
				atomic.AddUint64(&ps.TotalUses, 1)
				return ps.URL, nil
			}
			continue // 仍在冷卻中，跳過
		}

		atomic.AddUint64(&ps.TotalUses, 1)
		return ps.URL, nil
	}

	return "", fmt.Errorf("所有 Proxy (%d 個) 皆處於冷卻/封鎖狀態", n)
}

// ReportFailure 當請求遇到 403 / 429 或網路錯誤時呼叫，自動進行冷卻隔離
func (pm *ProxyManager) ReportFailure(proxyURL string, statusCode int) {
	if proxyURL == "" {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, ps := range pm.proxies {
		if ps.URL == proxyURL {
			ps.FailCount++
			ps.Banned = true
			ps.BannedAt = time.Now()
			log.Printf("[WARN] [ProxyManager] Proxy 異常 (狀態碼: %d | 連續失敗: %d 次) -> 已進入冷卻隔離 %v: %s",
				statusCode, ps.FailCount, ps.Cooldown, ps.URL)
			return
		}
	}
}

// ReportSuccess 當請求成功時重置失敗計數
func (pm *ProxyManager) ReportSuccess(proxyURL string) {
	if proxyURL == "" {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, ps := range pm.proxies {
		if ps.URL == proxyURL {
			ps.FailCount = 0
			ps.Banned = false
			return
		}
	}
}

// TotalCount 取得總代理數量
func (pm *ProxyManager) TotalCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.proxies)
}

// ActiveCount 取得當前可用代理數量
func (pm *ProxyManager) ActiveCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	now := time.Now()
	active := 0
	for _, ps := range pm.proxies {
		if !ps.Banned || now.Sub(ps.BannedAt) >= ps.Cooldown {
			active++
		}
	}
	return active
}
