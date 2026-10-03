package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"payroute/internal/channel"
	"payroute/internal/paypal"
	"payroute/internal/risk"
	"payroute/internal/router"
)

// Payment 订单
type Payment struct {
	ID        string            `json:"id"`
	Amount    float64           `json:"amount"`
	Currency  string            `json:"currency"`
	IP        string            `json:"ip"`
	CardNo    string            `json:"card_no"`
	Status    string            `json:"status"` // success / failed / blocked
	ChannelID string            `json:"channel_id"`
	Attempts  int               `json:"attempts"`
	Risk      risk.RiskResult   `json:"risk"`
	Route     router.RouteResult `json:"route"`
	LatencyMs int               `json:"latency_ms"`
	CreatedAt time.Time         `json:"created_at"`
}

var (
	mu       sync.Mutex
	payments = []Payment{}
	seq      = 0
)

type payRequest struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	IP       string  `json:"ip"`
	CardNo   string  `json:"card_no"`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// CreatePayment POST /api/v1/pay
func CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req payRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "amount must be > 0", 400)
		return
	}
	if req.Currency == "" {
		req.Currency = "CNY"
	}
	if req.IP == "" {
		req.IP = r.RemoteAddr
	}

	// 1. 风控
	rk := risk.Evaluate(req.Amount, req.IP, req.CardNo)

	mu.Lock()
	seq++
	id := fmt.Sprintf("pay_%d_%d", time.Now().Unix(), seq)
	mu.Unlock()

	p := Payment{
		ID: id, Amount: req.Amount, Currency: req.Currency,
		IP: req.IP, CardNo: maskCard(req.CardNo),
		Risk: rk, CreatedAt: time.Now(),
	}

	if rk.Blocked {
		p.Status = "blocked"
		save(p)
		writeJSON(w, p)
		return
	}

	// 2. 智能路由
	route := router.Select(req.Amount)
	p.Route = route

	// 3. 依次尝试主 + 备用通道
	candidates := append([]channel.Channel{route.Primary}, route.Fallbacks...)
	success := false
	for i, ch := range candidates {
		ok, latency := ch.Process(req.Amount)
		p.Attempts = i + 1
		p.LatencyMs += latency
		if ok {
			success = true
			p.ChannelID = ch.ID
			break
		}
		p.ChannelID = ch.ID // 记录最后尝试的通道
	}
	if success {
		p.Status = "success"
	} else {
		p.Status = "failed"
	}
	save(p)
	writeJSON(w, p)
}

// CreatePayPalOrder POST /api/v1/paypal/order - 创建 PayPal 沙箱订单
func CreatePayPalOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "amount must be > 0", 400)
		return
	}
	if req.Currency == "" {
		req.Currency = "USD" // PayPal 沙箱常用 USD
	}
	client := paypal.NewFromEnv()
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	base := scheme + "://" + r.Host
	order, err := client.CreateOrder(req.Amount, req.Currency, base+"/success", base+"/cancel")
	if err != nil {
		http.Error(w, "paypal error: "+err.Error(), 502)
		return
	}
	writeJSON(w, map[string]any{
		"order":      order,
		"configured": client.Configured(),
		"hint":       "未配置 PAYPAL_CLIENT_ID 时为模拟模式；配置后走真实沙箱",
	})
}

// ListPayments GET /api/v1/payments
func ListPayments(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	// 倒序
	out := make([]Payment, len(payments))
	for i, p := range payments {
		out[len(payments)-1-i] = p
	}
	if len(out) > 50 {
		out = out[:50]
	}
	writeJSON(w, map[string]any{"payments": out, "total": len(payments)})
}

// ListChannels GET /api/v1/channels
func ListChannels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"channels": channel.List()})
}

// Stats GET /api/v1/stats
func Stats(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	total := len(payments)
	succ, blocked, failed := 0, 0, 0
	var volume float64
	for _, p := range payments {
		volume += p.Amount
		switch p.Status {
		case "success":
			succ++
		case "blocked":
			blocked++
		case "failed":
			failed++
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(succ) / float64(total) * 100
	}
	writeJSON(w, map[string]any{
		"total": total, "success": succ, "blocked": blocked, "failed": failed,
		"success_rate": fmt.Sprintf("%.1f%%", rate), "volume": volume,
	})
}

func save(p Payment) {
	mu.Lock()
	defer mu.Unlock()
	payments = append(payments, p)
}

func maskCard(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "**** **** **** " + s[len(s)-4:]
}
