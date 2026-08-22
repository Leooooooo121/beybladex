package sites

import (
	"context"
	"fmt"
	"sync"
	"time"

	"beyblade/config"
	"beyblade/models"
	"beyblade/proxy"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// SiteMonitor 網站監控器介面 (Strategy Pattern)
type SiteMonitor interface {
	TaskID() string
	Name() string
	Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error
	CheckStock(ctx context.Context) ([]models.ProductStatus, error)
}

// MonitorFactory 監控器建構函式型別
type MonitorFactory func() SiteMonitor

var (
	registryMu sync.RWMutex
	registry   = make(map[string]MonitorFactory)
)

// RegisterMonitor 註冊適配器工廠
func RegisterMonitor(siteType string, factory MonitorFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[siteType] = factory
}

// CreateMonitor 依據 TaskConfig 的 SiteType 實例化對應的監控器
func CreateMonitor(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) (SiteMonitor, error) {
	registryMu.RLock()
	factory, exists := registry[task.SiteType]
	registryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("找不到未知的網站適配器類型: %s", task.SiteType)
	}

	monitor := factory()
	if err := monitor.Init(task, proxyMgr, timeout); err != nil {
		return nil, fmt.Errorf("初始化適配器 [%s] 失敗: %w", task.Name, err)
	}

	return monitor, nil
}

// BuildTLSClient 建置具備真實瀏覽器指紋 (Chrome 120+) 的 TLS Client
func BuildTLSClient(proxyURL string, timeout time.Duration) (tls_client.HttpClient, error) {
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithTimeoutSeconds(int(timeout.Seconds())),
	}

	if proxyURL != "" {
		options = append(options, tls_client.WithProxyUrl(proxyURL))
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("建立 TLS Client 失敗: %w", err)
	}

	return client, nil
}
