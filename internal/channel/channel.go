package channel

import (
	"math/rand"
	"time"
)

// Channel 支付通道接口
type Channel struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	FeeRate     float64 `json:"fee_rate"`     // 费率，如 0.006 表示 0.6%
	AvgLatency  int     `json:"avg_latency"`  // 平均延迟 ms
	SuccessRate float64 `json:"success_rate"` // 成功率 0-1
	Enabled     bool    `json:"enabled"`
}

// 模拟通道库
var channels = []Channel{
	{ID: "paypal", Name: "PayPal", FeeRate: 0.029, AvgLatency: 280, SuccessRate: 0.985, Enabled: true},
	{ID: "stripe", Name: "Stripe", FeeRate: 0.029, AvgLatency: 320, SuccessRate: 0.98, Enabled: true},
	{ID: "alipay", Name: "Alipay", FeeRate: 0.006, AvgLatency: 180, SuccessRate: 0.97, Enabled: true},
	{ID: "wechat", Name: "WeChat Pay", FeeRate: 0.006, AvgLatency: 200, SuccessRate: 0.96, Enabled: true},
	{ID: "unionpay", Name: "UnionPay", FeeRate: 0.005, AvgLatency: 450, SuccessRate: 0.93, Enabled: true},
}

func List() []Channel {
	return channels
}

func Get(id string) *Channel {
	for i := range channels {
		if channels[i].ID == id {
			return &channels[i]
		}
	}
	return nil
}

// Process 模拟发起支付，返回是否成功、实际延迟
func (c *Channel) Process(amount float64) (bool, int) {
	// 模拟延迟抖动
	latency := c.AvgLatency + rand.Intn(120) - 60
	if latency < 50 {
		latency = 50
	}
	time.Sleep(time.Duration(0)) // 不真实 sleep，保持 API 快速
	// 大额订单成功率略降，模拟风控
	rate := c.SuccessRate
	if amount > 10000 {
		rate -= 0.05
	}
	return rand.Float64() < rate, latency
}

// Score 通道打分，分数越高越优
func (c *Channel) Score(amount float64) float64 {
	if !c.Enabled {
		return -1
	}
	feeCost := amount * c.FeeRate
	// 成功率权重最高，费率和延迟作为惩罚项
	return c.SuccessRate*100 - feeCost*0.1 - float64(c.AvgLatency)*0.01
}
