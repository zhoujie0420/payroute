# ⚡ PayRoute - AI-Powered Smart Payment Routing

> PayPal AI Hackathon 2026 参赛作品 | 主题：Build What's Next with PayPal and AI

## 一句话
Golang 智能支付路由网关：一个 API 聚合 PayPal 沙箱 + 多通道，AI 实时风控，可解释、可演示。

## 痛点
小商户接多个支付渠道成本高：单通道故障就掉单，费率高低不一，没有统一的风控。AI 时代，支付更需要智能决策层。

## 解决方案：PayPal + AI 双核心

### 1. PayPal 深度集成（满足"meaningfully integrate PayPal"）
- `POST /api/v1/paypal/order`：直连 PayPal Sandbox Orders API v2（OAuth2 + 创建订单 + 返回 approve_url）
- 未配置 `PAYPAL_CLIENT_ID` 时自动降级为模拟模式，保证评委一键可跑
- 配置后走真实沙箱：`PAYPAL_CLIENT_ID` / `PAYPAL_CLIENT_SECRET` / `PAYPAL_SANDBOX=true`
- PayPal 作为智能路由的优选通道之一（成功率 98.5%）

### 2. AI 风控（满足"meaningfully incorporate AI"）
自研 `payroute-fraud-v2` 模型：特征工程 + 加权逻辑回归 + 自适应异常检测
- 特征：金额 z-score（自适应基线）、IP/卡号双维度频次、深夜时段、卡号可疑模式、黑名单
- 输出：0-100 风险分 + 等级 + **每个特征的贡献度**（可解释 AI）
- 80 分自动熔断拦截
- 每次返回都带 `ai_model` 和 `factors`，评委可验证

### 3. 智能路由
按 `成功率×100 - 费率成本 - 延迟惩罚` 动态打分，主通道失败自动切换备用（演示中可见 attempts=2）

## 技术栈
- 后端：Golang（标准库 net/http，零重型依赖）
- AI：自研风控模型（Go 原生实现，无需 Python 服务）
- 前端：原生 Dashboard，实时看板
- 部署：Docker 一键部署

## 快速运行（评委 3 分钟可复现）

```bash
# 1. 启动
go run ./cmd/server
# 打开 http://localhost:8080

# 2. 发起智能路由支付
curl -X POST http://localhost:8080/api/v1/pay \
  -H "Content-Type: application/json" \
  -d '{"amount":299,"currency":"CNY","ip":"203.0.113.10","card_no":"6222021234567890"}'

# 3. 创建 PayPal 订单（模拟模式，无需凭证）
curl -X POST http://localhost:8080/api/v1/paypal/order \
  -H "Content-Type: application/json" \
  -d '{"amount":49.99,"currency":"USD"}'

# 4. 看 AI 风控拦截（测试卡 BIN）
curl -X POST http://localhost:8080/api/v1/pay \
  -H "Content-Type: application/json" \
  -d '{"amount":9999,"ip":"203.0.113.99","card_no":"4111111111111111"}'
# 返回 risk.score、level、factors 可解释
```

### 接真实 PayPal 沙箱
1. 去 https://developer.paypal.com 注册拿 Sandbox 的 Client ID / Secret
2. `export PAYPAL_CLIENT_ID=xxx PAYPAL_CLIENT_SECRET=yyy`
3. 重启服务，`/api/v1/paypal/order` 即返回真实 approve_url

## API
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/pay | 智能路由支付（含 AI 风控） |
| POST | /api/v1/paypal/order | 创建 PayPal 沙箱订单 |
| GET | /api/v1/payments | 最近订单 |
| GET | /api/v1/channels | 通道状态（含 PayPal） |
| GET | /api/v1/stats | 统计 |

## 演示脚本（3 分钟）
0:00 痛点：小商户多通道接入难 → 0:30 Dashboard 展示 5 通道 → 1:00 发起支付看智能路由+重试 → 1:40 PayPal 订单创建（mock→真实） → 2:10 AI 风控：正常 17 分 vs 可疑 43 分，展示 factors 可解释性 → 2:40 总结：PayPal 支付层 + AI 决策层

## 商业化
- 目标：月交易 100 万以下小微商户 / AI 应用开发者
- 收费：按节省手续费的 20% 分成
- 扩展：已预留 Stripe/Adyen 接口，2 周可接更多通道

## 团队
- elk（个人参赛）| jiezhou8917@gmail.com
- 技术栈：Golang，每周 10 小时，6 周交付
