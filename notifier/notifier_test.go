package notifier

import (
	"testing"
	"time"

	"beyblade/config"
	"beyblade/models"
)

func TestConsoleNotifier_Send(t *testing.T) {
	cn := NewConsoleNotifier()
	status := models.ProductStatus{
		SiteName:    "TestShop",
		ProductID:   "101",
		Title:       "Beyblade X Test",
		VariantName: "Spec A",
		Price:       295,
		Currency:    "TWD",
		Quantity:    3,
		URL:         "https://example.com/item/101",
		Timestamp:   time.Now(),
	}

	err := cn.Send(status)
	if err != nil {
		t.Fatalf("ConsoleNotifier.Send 失敗: %v", err)
	}
}

func TestMultiNotifier_Init(t *testing.T) {
	cfg := config.NotifiersConfig{
		Console:           true,
		DiscordWebhookURL: "https://discord.com/api/webhooks/test",
		TelegramBotToken:  "test_token",
		TelegramChatID:    "test_chat_id",
	}

	mn := NewMultiNotifier(cfg)
	if len(mn.notifiers) != 3 {
		t.Errorf("預期 3 個通知器，得到 %d", len(mn.notifiers))
	}
}
