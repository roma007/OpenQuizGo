package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	base := flag.String("base", "http://localhost:8080", "服务地址")
	path := flag.String("path", "/v1/child/context", "压测路径")
	token := flag.String("token", "", "设备令牌（可选）")
	rate := flag.Int("rate", 1000, "目标 QPS")
	duration := flag.Duration("duration", 10*time.Second, "持续时间")
	concurrency := flag.Int("concurrency", 200, "最大并发")
	flag.Parse()

	if *rate <= 0 || *concurrency <= 0 {
		fmt.Fprintln(os.Stderr, "rate and concurrency must be positive")
		os.Exit(1)
	}

	url := strings.TrimRight(*base, "/") + *path
	transport := &http.Transport{
		MaxIdleConns:        *concurrency * 2,
		MaxIdleConnsPerHost: *concurrency * 2,
		MaxConnsPerHost:     *concurrency * 2,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, *rate*int(duration.Seconds()))
	errorsCount, statusErrors, dropped := 0, 0, 0

	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()

	start := time.Now()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			select {
			case sem <- struct{}{}:
			default:
				dropped++
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				req, err := http.NewRequest(http.MethodGet, url, nil)
				if err != nil {
					mu.Lock()
					errorsCount++
					mu.Unlock()
					return
				}
				if *token != "" {
					req.Header.Set("Authorization", "Bearer "+*token)
				}
				reqStart := time.Now()
				resp, err := client.Do(req)
				elapsed := time.Since(reqStart)

				mu.Lock()
				latencies = append(latencies, elapsed)
				if err != nil {
					errorsCount++
				} else if resp.StatusCode >= 400 {
					statusErrors++
				}
				mu.Unlock()

				if resp != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
				}
			}()
		}
	}
	wg.Wait()
	elapsed := time.Since(start)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	total := len(latencies)
	fmt.Printf("url=%s total=%d duration=%s qps=%.1f dropped=%d network_errors=%d http_errors=%d\n",
		url, total, elapsed.Round(time.Millisecond),
		float64(total)/elapsed.Seconds(), dropped, errorsCount, statusErrors)
	if total > 0 {
		fmt.Printf("latency p50=%s p95=%s p99=%s max=%s\n",
			percentile(latencies, 0.50), percentile(latencies, 0.95),
			percentile(latencies, 0.99), latencies[total-1])
	}
	if errorsCount > 0 || statusErrors > 0 {
		os.Exit(1)
	}
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}
