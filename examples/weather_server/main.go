package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type weatherReq struct {
	City string `json:"city" description:"city name for weather query"`
}

type weatherResponse struct {
	Weather     string `json:"weather"`
	Temperature string `json:"temperature"`
	Humidity    string `json:"humidity"`
	WindSpeed   string `json:"wind_speed"`
}

func main() {
	// new mcp server with stdio or sse transport
	srv, err := server.NewServer(
		getTransport(),
		server.WithServerInfo(protocol.Implementation{
			Name:    "weather-server",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// new protocol tool with name, description and properties
	tool, err := protocol.NewTool("get_weather", "Get current weather information for a specified city", weatherReq{})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
		return
	}

	// register tool and start mcp server
	srv.RegisterTool(tool, getWeather)

	errCh := make(chan error)
	go func() {
		errCh <- srv.Run()
	}()

	if err = signalWaiter(errCh); err != nil {
		log.Fatalf("signal waiter: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
}

func getTransport() (t transport.ServerTransport) {
	var (
		mode string
		addr = "127.0.0.1:8080"
	)

	flag.StringVar(&mode, "transport", "stdio", "The transport to use, should be \"stdio\" or \"sse\"")
	flag.Parse()

	if mode == "stdio" {
		log.Println("start weather mcp server with stdio transport")
		t = transport.NewStdioServerTransport()
	} else {
		log.Printf("start weather mcp server with sse transport, listen %s", addr)
		t, _ = transport.NewSSEServerTransport(addr)
	}

	return t
}

func getWeather(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	req := new(weatherReq)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, &req); err != nil {
		return nil, err
	}

	// 新增环境变量配置
	var (
		apiKey = os.Getenv("WEATHER_API_KEY")
	)

	// 修改API调用部分
	if apiKey == "" {
		return nil, fmt.Errorf("WEATHER_API_KEY环境变量未配置")
	}
	apiURL := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no", apiKey, req.City)

	response, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API请求失败: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("API返回错误状态码: %d", response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// 解析API响应
	type apiResponse struct {
		Current struct {
			TempC     float64 `json:"temp_c"`
			Humidity  int     `json:"humidity"`
			WindKph   float64 `json:"wind_kph"`
			Condition struct {
				Text string `json:"text"`
			} `json:"condition"`
		} `json:"current"`
	}

	var apiRes apiResponse
	if err := json.Unmarshal(body, &apiRes); err != nil {
		return nil, fmt.Errorf("响应解析失败: %v", err)
	}

	// 模拟天气数据（实际使用时替换为真实API数据）
	weatherInfo := weatherResponse{
		Weather:     "晴天",
		Temperature: "25°C",
		Humidity:    "65%",
		WindSpeed:   "3.5 m/s",
	}

	text := fmt.Sprintf("当前%s的天气情况：\n天气：%s\n温度：%s\n湿度：%s\n风速：%s",
		req.City, weatherInfo.Weather, weatherInfo.Temperature, weatherInfo.Humidity, weatherInfo.WindSpeed)

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			protocol.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}, nil
}

func signalWaiter(errCh chan error) error {
	signalToNotify := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM}
	if signal.Ignored(syscall.SIGHUP) {
		signalToNotify = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, signalToNotify...)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
			log.Printf("Received signal: %s\n", sig)
			// graceful shutdown
			return nil
		}
	case err := <-errCh:
		return err
	}

	return nil
}
