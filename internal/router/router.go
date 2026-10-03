package router

import (
	"sort"

	"payroute/internal/channel"
)

// RouteResult 路由结果
type RouteResult struct {
	Primary   channel.Channel   `json:"primary"`
	Fallbacks []channel.Channel `json:"fallbacks"`
}

// Select 智能路由：按打分排序，返回主通道 + 备用通道
func Select(amount float64) RouteResult {
	all := channel.List()
	scored := make([]channel.Channel, 0, len(all))
	for _, c := range all {
		if c.Enabled {
			scored = append(scored, c)
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score(amount) > scored[j].Score(amount)
	})
	if len(scored) == 0 {
		return RouteResult{}
	}
	fb := []channel.Channel{}
	if len(scored) > 1 {
		fb = scored[1:]
		if len(fb) > 2 {
			fb = fb[:2]
		}
	}
	return RouteResult{Primary: scored[0], Fallbacks: fb}
}
