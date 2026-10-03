package risk

import (
	"math"
	"strings"
	"sync"
	"time"
)

// RiskResult 风控评估结果
type RiskResult struct {
	Score   int            `json:"score"` // 0-100，越高越危险
	Level   string         `json:"level"` // low / medium / high
	Reason  string         `json:"reason"`
	Blocked bool           `json:"blocked"`
	AIModel string         `json:"ai_model,omitempty"`
	Factors []FactorWeight `json:"factors,omitempty"`
}

// FactorWeight 特征贡献度（可解释 AI）
type FactorWeight struct {
	Feature      string  `json:"feature"`
	Contribution float64 `json:"contribution"` // 对风险分的贡献
	Detail       string  `json:"detail"`
}

var (
	mu            sync.Mutex
	ipCount       = map[string][]time.Time{}
	cardCount     = map[string][]time.Time{}
	blacklistIP   = map[string]bool{"192.168.99.99": true}
	blacklistCard = []string{"411111"}

	// 历史交易金额分布（用于异常检测的自适应基线）
	amountHistory []float64
	amountMu      sync.Mutex
)

func recordAmount(a float64) (mean, std float64) {
	amountMu.Lock()
	defer amountMu.Unlock()
	amountHistory = append(amountHistory, a)
	if len(amountHistory) > 1000 {
		amountHistory = amountHistory[len(amountHistory)-1000:]
	}
	n := float64(len(amountHistory))
	if n == 0 {
		return a, 1
	}
	sum := 0.0
	for _, v := range amountHistory {
		sum += v
	}
	mean = sum / n
	var sq float64
	for _, v := range amountHistory {
		sq += (v - mean) * (v - mean)
	}
	std = math.Sqrt(sq / n)
	if std < 1 {
		std = 1
	}
	return mean, std
}

// sigmoid
func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// AIEvaluate AI 增强的风控评估：特征工程 + 加权逻辑回归 + 异常检测
func AIEvaluate(amount float64, ip, cardNo string, hour int) RiskResult {
	factors := []FactorWeight{}
	logit := -2.5 // 偏置项：默认偏向低风险

	add := func(feature string, w float64, detail string) {
		logit += w
		factors = append(factors, FactorWeight{Feature: feature, Contribution: w, Detail: detail})
	}

	// 1. 黑名单（强规则）
	if blacklistIP[ip] {
		return RiskResult{Score: 100, Level: "high", Reason: "IP 黑名单", Blocked: true, AIModel: "payroute-fraud-v2", Factors: factors}
	}
	for _, b := range blacklistCard {
		if strings.HasPrefix(cardNo, b) {
			add("blacklist_card_bin", 2.2, "命中测试卡 BIN")
		}
	}

	// 2. 金额异常检测（z-score，自适应基线）
	mean, std := recordAmount(amount)
	z := (amount - mean) / std
	if z > 3 {
		add("amount_anomaly", 1.8, "金额超过均值 3 个标准差")
	} else if z > 2 {
		add("amount_anomaly", 1.0, "金额超过均值 2 个标准差")
	} else if amount > 50000 {
		add("amount_large", 1.2, "超大额交易")
	} else if amount > 10000 {
		add("amount_large", 0.6, "大额交易")
	}

	// 3. 频次特征（IP + 卡号双维度滑动窗口）
	mu.Lock()
	now := time.Now()
	ipTs := prune(ipCount[ip], now)
	ipTs = append(ipTs, now)
	ipCount[ip] = ipTs
	cardTs := prune(cardCount[cardNo], now)
	cardTs = append(cardTs, now)
	cardCount[cardNo] = cardTs
	mu.Unlock()

	if len(ipTs) > 10 {
		add("velocity_ip", 1.6, "同一 IP 1 分钟内超 10 次")
	} else if len(ipTs) > 5 {
		add("velocity_ip", 0.8, "同一 IP 1 分钟内超 5 次")
	}
	if len(cardTs) > 8 {
		add("velocity_card", 1.4, "同一卡号 1 分钟内超 8 次")
	} else if len(cardTs) > 4 {
		add("velocity_card", 0.7, "同一卡号 1 分钟内超 4 次")
	}

	// 4. 时间特征（深夜交易风险略高）
	if hour < 0 {
		hour = now.Hour()
	}
	if hour >= 2 && hour <= 5 {
		add("time_night", 0.5, "深夜时段交易")
	}

	// 5. 卡号模式（重复数字、顺序号）
	if hasSuspiciousPattern(cardNo) {
		add("card_pattern", 0.9, "卡号存在可疑模式")
	}

	prob := sigmoid(logit)
	score := int(math.Round(prob * 100))

	level := "low"
	blocked := false
	if score >= 80 {
		level = "high"
		blocked = true
	} else if score >= 45 {
		level = "medium"
	}

	reasons := []string{}
	for _, f := range factors {
		if f.Contribution >= 0.5 {
			reasons = append(reasons, f.Detail)
		}
	}
	reason := "正常"
	if len(reasons) > 0 {
		reason = strings.Join(reasons, "; ")
	}
	return RiskResult{
		Score: score, Level: level, Reason: reason, Blocked: blocked,
		AIModel: "payroute-fraud-v2", Factors: factors,
	}
}

func prune(ts []time.Time, now time.Time) []time.Time {
	out := []time.Time{}
	for _, t := range ts {
		if now.Sub(t) < time.Minute {
			out = append(out, t)
		}
	}
	return out
}

func hasSuspiciousPattern(card string) bool {
	digits := ""
	for _, c := range card {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
	}
	if len(digits) < 8 {
		return false
	}
	// 全相同数字
	same := true
	for i := 1; i < len(digits); i++ {
		if digits[i] != digits[0] {
			same = false
			break
		}
	}
	if same {
		return true
	}
	// 连续递增/递减 6 位
	for i := 0; i+6 <= len(digits); i++ {
		asc, desc := true, true
		for j := i + 1; j < i+6; j++ {
			if digits[j] != digits[j-1]+1 {
				asc = false
			}
			if digits[j] != digits[j-1]-1 {
				desc = false
			}
		}
		if asc || desc {
			return true
		}
	}
	return false
}

// Evaluate 保留旧接口，内部走 AI 模型
func Evaluate(amount float64, ip, cardNo string) RiskResult {
	return AIEvaluate(amount, ip, cardNo, -1)
}
