package paypal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Client PayPal 沙箱客户端
type Client struct {
	ClientID     string
	ClientSecret string
	Sandbox      bool
	httpClient   *http.Client

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func NewFromEnv() *Client {
	return &Client{
		ClientID:     os.Getenv("PAYPAL_CLIENT_ID"),
		ClientSecret: os.Getenv("PAYPAL_CLIENT_SECRET"),
		Sandbox:      os.Getenv("PAYPAL_SANDBOX") != "false",
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured 是否配置了真实凭证
func (c *Client) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

func (c *Client) baseURL() string {
	if c.Sandbox {
		return "https://api-m.sandbox.paypal.com"
	}
	return "https://api-m.paypal.com"
}

// accessToken 获取（带缓存）
func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(tokenExpiryMargin(c.tokenExpiry)) {
		return c.token, nil
	}
	req, _ := http.NewRequest("POST", c.baseURL()+"/v1/oauth2/token",
		bytes.NewBufferString("grant_type=client_credentials"))
	auth := base64.StdEncoding.EncodeToString([]byte(c.ClientID + ":" + c.ClientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("paypal token failed: %d %s", resp.StatusCode, string(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	c.token = out.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	return c.token, nil
}

func tokenExpiryMargin(t time.Time) time.Time { return t.Add(-60 * time.Second) }

// OrderResult 下单结果
type OrderResult struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	ApproveURL string `json:"approve_url"`
	Mock       bool   `json:"mock"` // true 表示未配置凭证时的模拟模式
}

// CreateOrder 创建 PayPal 订单
func (c *Client) CreateOrder(amount float64, currency, returnURL, cancelURL string) (*OrderResult, error) {
	// 未配置凭证时返回模拟结果，保证 Demo 可跑
	if !c.Configured() {
		return &OrderResult{
			ID:         fmt.Sprintf("MOCK-%d", time.Now().Unix()),
			Status:     "CREATED",
			ApproveURL: "https://www.sandbox.paypal.com/checkoutnow?token=MOCK",
			Mock:       true,
		}, nil
	}
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{
			{
				"amount": map[string]string{
					"currency_code": currency,
					"value":         fmt.Sprintf("%.2f", amount),
				},
			},
		},
		"application_context": map[string]string{
			"return_url":  returnURL,
			"cancel_url":  cancelURL,
			"brand_name":  "PayRoute",
			"user_action": "PAY_NOW",
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", c.baseURL()+"/v2/checkout/orders", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("paypal order failed: %d %s", resp.StatusCode, string(b))
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Links  []struct {
			Href   string `json:"href"`
			Rel    string `json:"rel"`
			Method string `json:"method"`
		} `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	approve := ""
	for _, l := range out.Links {
		if l.Rel == "approve" {
			approve = l.Href
		}
	}
	return &OrderResult{ID: out.ID, Status: out.Status, ApproveURL: approve}, nil
}
