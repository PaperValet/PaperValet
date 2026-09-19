package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type WeatherPlugin struct {
	http *http.Client
}

func New() *WeatherPlugin {
	return &WeatherPlugin{http: &http.Client{Timeout: 15 * time.Second}}
}

func (p *WeatherPlugin) Name() string        { return "weather" }
func (p *WeatherPlugin) Description() string { return "天气查询" }

func (p *WeatherPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "weather",
		Aliases:     []string{"天气", "wt"},
		Description: "查询天气（数据源 wttr.in）",
		Usage:       "weather <城市>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleWeather,
	})
}

func (p *WeatherPlugin) Start(ctx context.Context) error { return nil }
func (p *WeatherPlugin) Stop(ctx context.Context) error  { return nil }

type wttrResp struct {
	CurrentCondition []struct {
		TempC      string `json:"temp_C"`
		FeelsLikeC string `json:"FeelsLikeC"`
		Humidity   string `json:"humidity"`
		WindKmph   string `json:"windspeedKmph"`
		WindDir    string `json:"winddir16Point"`
		LangZh     []struct {
			Value string `json:"value"`
		} `json:"lang_zh"`
		WeatherDesc []struct {
			Value string `json:"value"`
		} `json:"weatherDesc"`
	} `json:"current_condition"`
	NearestArea []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
		Country []struct {
			Value string `json:"value"`
		} `json:"country"`
	} `json:"nearest_area"`
	Weather []struct {
		Date    string `json:"date"`
		MaxTempC string `json:"maxtempC"`
		MinTempC string `json:"mintempC"`
	} `json:"weather"`
}

func (p *WeatherPlugin) handleWeather(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit("🌤 用法：<code>weather <城市></code>，例如 <code>weather 北京</code>")
	}
	city := strings.Join(ctx.Args, " ")

	reqURL := "https://wttr.in/" + url.PathEscape(city) + "?format=j1&lang=zh"
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet, reqURL, nil)
	if err != nil {
		return ctx.Edit("❌ 请求构建失败")
	}
	req.Header.Set("User-Agent", "curl/8")
	resp, err := p.http.Do(req)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 天气查询失败: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ctx.Edit(fmt.Sprintf("❌ 天气服务返回 %d", resp.StatusCode))
	}

	var data wttrResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.CurrentCondition) == 0 {
		return ctx.Edit("❌ 未找到该城市的天气数据")
	}

	cur := data.CurrentCondition[0]
	desc := ""
	if len(cur.LangZh) > 0 {
		desc = cur.LangZh[0].Value
	} else if len(cur.WeatherDesc) > 0 {
		desc = cur.WeatherDesc[0].Value
	}
	area := city
	if len(data.NearestArea) > 0 && len(data.NearestArea[0].AreaName) > 0 {
		area = data.NearestArea[0].AreaName[0].Value
		if len(data.NearestArea[0].Country) > 0 {
			area += ", " + data.NearestArea[0].Country[0].Value
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🌤 <b>%s</b>\n\n", area)
	fmt.Fprintf(&sb, "天气：%s\n气温：%s°C（体感 %s°C）\n湿度：%s%%\n风：%s %s km/h\n",
		desc, cur.TempC, cur.FeelsLikeC, cur.Humidity, cur.WindDir, cur.WindKmph)

	if len(data.Weather) > 0 {
		sb.WriteString("\n<b>未来预报</b>\n")
		for i, d := range data.Weather {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&sb, "%s：%s ~ %s°C\n", d.Date, d.MinTempC, d.MaxTempC)
		}
	}
	return ctx.Edit(sb.String())
}
