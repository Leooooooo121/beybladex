package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"beyblade/config"
	"beyblade/models"
)

// Notifier 通知發送介面
type Notifier interface {
	Name() string
	Send(status models.ProductStatus) error
}

// MultiNotifier 聚合多個通知器並行廣播
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier 依據設定檔與環境變數建立啟用的通知模組
func NewMultiNotifier(cfg config.NotifiersConfig) *MultiNotifier {
	mn := &MultiNotifier{}

	if cfg.Console {
		mn.notifiers = append(mn.notifiers, NewConsoleNotifier())
	}

	discordURL := cfg.DiscordWebhookURL
	if envURL := os.Getenv("DISCORD_WEBHOOK_URL"); envURL != "" {
		discordURL = envURL
	}
	if discordURL != "" {
		mn.notifiers = append(mn.notifiers, NewDiscordNotifier(discordURL))
	}

	tgToken := cfg.TelegramBotToken
	if envToken := os.Getenv("TELEGRAM_BOT_TOKEN"); envToken != "" {
		tgToken = envToken
	}
	tgChatID := cfg.TelegramChatID
	if envChat := os.Getenv("TELEGRAM_CHAT_ID"); envChat != "" {
		tgChatID = envChat
	}

	if tgToken != "" && tgChatID != "" {
		mn.notifiers = append(mn.notifiers, NewTelegramNotifier(tgToken, tgChatID))
	}

	return mn
}

func (m *MultiNotifier) Name() string {
	return "MultiNotifier"
}

func (m *MultiNotifier) Send(status models.ProductStatus) error {
	var wg sync.WaitGroup
	for _, n := range m.notifiers {
		wg.Add(1)
		go func(target Notifier) {
			defer wg.Done()
			if err := target.Send(status); err != nil {
				log.Printf("[ERROR] [Notifier:%s] 發送通知失敗: %v", target.Name(), err)
			}
		}(n)
	}
	wg.Wait()
	return nil
}

// StartListener 啟動事件監聽器，持續消費 Channel 訊息
func StartListener(ctx context.Context, ch <-chan models.ProductStatus, notifier Notifier) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case status, ok := <-ch:
				if !ok {
					return
				}
				_ = notifier.Send(status)
			}
		}
	}()
}

// =========================================================================
// 1. Console Notifier (終端機高精度毫秒日誌)
// =========================================================================

type ConsoleNotifier struct{}

func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{}
}

func (c *ConsoleNotifier) Name() string {
	return "Console"
}

func (c *ConsoleNotifier) Send(status models.ProductStatus) error {
	ts := status.Timestamp.Format("2006-01-02 15:04:05.000")
	fmt.Println("\n=======================================================")
	fmt.Printf("⚡ [%s] 【🔥 補貨/有庫存通知】\n", ts)
	fmt.Printf("🏪 來源平台: %s\n", status.SiteName)
	fmt.Printf("📦 商品名稱: %s\n", status.Title)
	if status.VariantName != "" {
		fmt.Printf("🏷️ 規格款式: %s\n", status.VariantName)
	}
	fmt.Printf("💰 價格資訊: %s %.2f\n", status.Currency, status.Price)
	fmt.Printf("📊 庫存數量: %d\n", status.Quantity)
	fmt.Printf("🔗 直達連結: %s\n", status.URL)
	fmt.Println("=======================================================")
	return nil
}

// =========================================================================
// 2. Discord Webhook Notifier
// =========================================================================

type DiscordNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (d *DiscordNotifier) Name() string {
	return "Discord"
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	URL         string         `json:"url"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields"`
	Thumbnail   *discordImage  `json:"thumbnail,omitempty"`
	Timestamp   string         `json:"timestamp"`
	Footer      *discordFooter `json:"footer,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordImage struct {
	URL string `json:"url"`
}

type discordFooter struct {
	Text string `json:"text"`
}

type discordPayload struct {
	Username  string         `json:"username"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []discordEmbed `json:"embeds"`
}

func (d *DiscordNotifier) Send(status models.ProductStatus) error {
	embed := discordEmbed{
		Title:       fmt.Sprintf("🚨 補貨通知: %s", status.Title),
		Description: fmt.Sprintf("[%s](%s)", status.Title, status.URL),
		URL:         status.URL,
		Color:       3066993, // Emerald Green
		Timestamp:   status.Timestamp.Format(time.RFC3339),
		Fields: []discordField{
			{Name: "來源", Value: status.SiteName, Inline: true},
			{Name: "規格", Value: status.VariantName, Inline: true},
			{Name: "價格", Value: fmt.Sprintf("%s %.2f", status.Currency, status.Price), Inline: true},
			{Name: "庫存數量", Value: fmt.Sprintf("%d", status.Quantity), Inline: true},
		},
		Footer: &discordFooter{
			Text: "High-Freq Restock Monitor (Golang)",
		},
	}

	if status.ImageURL != "" {
		embed.Thumbnail = &discordImage{URL: status.ImageURL}
	}

	payload := discordPayload{
		Username: "Restock Bot",
		Embeds:   []discordEmbed{embed},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := d.client.Post(d.webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord API 回應 HTTP %d", resp.StatusCode)
	}

	return nil
}

// =========================================================================
// 3. Telegram Bot Notifier (TG 推播模組)
// =========================================================================

type TelegramNotifier struct {
	token  string
	chatID string
	client *http.Client
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		token:  token,
		chatID: chatID,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (t *TelegramNotifier) Name() string {
	return "Telegram"
}

type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (t *TelegramNotifier) Send(status models.ProductStatus) error {
	ts := status.Timestamp.Format("2006-01-02 15:04:05.000")

	// HTML 特殊字元跳脫，避免包含 <, >, & 的商品名稱造成 Telegram HTML 解析失敗
	safeTitle := html.EscapeString(status.Title)
	safeVariant := html.EscapeString(status.VariantName)
	safeSite := html.EscapeString(status.SiteName)

	text := fmt.Sprintf(
		"⚡ <b>【補貨通知】</b>\n\n"+
			"🏪 <b>來源:</b> %s\n"+
			"📦 <b>商品:</b> %s\n"+
			"🏷️ <b>規格:</b> %s\n"+
			"💰 <b>價格:</b> %s %.2f\n"+
			"📊 <b>庫存:</b> %d\n"+
			"🕒 <b>時間:</b> %s\n"+
			"🔗 <a href=\"%s\">立即前往購買</a>",
		safeSite,
		safeTitle,
		safeVariant,
		status.Currency,
		status.Price,
		status.Quantity,
		ts,
		status.URL,
	)

	payload := telegramPayload{
		ChatID:    t.chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
	resp, err := t.client.Post(apiURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("發送至 Telegram 失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API 回應 HTTP %d", resp.StatusCode)
	}

	return nil
}
