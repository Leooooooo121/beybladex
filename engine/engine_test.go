package engine

import (
	"context"
	"testing"
	"time"

	"beyblade/config"
	"beyblade/models"
	"beyblade/proxy"
	"beyblade/sites"
)

type mockNotifier struct {
	received chan models.ProductStatus
}

func (m *mockNotifier) Name() string { return "MockNotifier" }
func (m *mockNotifier) Send(status models.ProductStatus) error {
	m.received <- status
	return nil
}

type mockMonitor struct {
	task config.TaskConfig
}

func (m *mockMonitor) TaskID() string { return m.task.ID }
func (m *mockMonitor) Name() string   { return m.task.Name }
func (m *mockMonitor) Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error {
	m.task = task
	return nil
}

var callCount int

func (m *mockMonitor) CheckStock(ctx context.Context) ([]models.ProductStatus, error) {
	callCount++
	inStock := callCount%2 == 1 // 輪流在有庫存與無庫存切換
	return []models.ProductStatus{
		{
			SiteName:    m.task.Name,
			ProductID:   "item-1",
			Title:       "Test Beyblade",
			VariantID:   "var-1",
			VariantName: "Normal",
			Price:       299,
			InStock:     inStock,
			Quantity:    1,
			Timestamp:   time.Now(),
		},
	}, nil
}

func TestEngine_LifecycleAndDeduplication(t *testing.T) {
	sites.RegisterMonitor("mock", func() sites.SiteMonitor {
		return &mockMonitor{}
	})

	cfg := &config.Config{
		Global: config.GlobalConfig{
			PollIntervalMinMs: 50,
			PollIntervalMaxMs: 100,
			TimeoutSeconds:    1,
			BackoffMinSec:     1,
			BackoffMaxSec:     2,
			ChannelBufferSize: 100,
		},
		Tasks: []config.TaskConfig{
			{
				ID:       "mock_task",
				SiteType: "mock",
				Name:     "Mock Task",
				Enabled:  true,
			},
		},
	}

	mockNotif := &mockNotifier{received: make(chan models.ProductStatus, 10)}
	pm := proxy.NewProxyManager([]string{}, 1*time.Second)

	eng, err := NewEngine(cfg, pm, mockNotif)
	if err != nil {
		t.Fatalf("建立 Engine 失敗: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = eng.Start(ctx)
		close(done)
	}()

	// 檢查是否收到通知
	select {
	case event := <-mockNotif.received:
		if event.ProductID != "item-1" || !event.InStock {
			t.Errorf("收到的事件內容異常: %+v", event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Errorf("逾時未收到通知事件")
	}

	<-done
}
