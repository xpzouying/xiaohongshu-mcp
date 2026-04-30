package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	fmt.Println("=== 使用 curl 中的签名头直接调用 album API ===")

	// 从用户提供的 curl 命令中提取的签名头
	xs := "XYS_2UQhPsHCH0c1PUhFHjIj2erjwjQhyoPTqBPT49pjHjIj2eHjwjQgynEDJ74AHjIj2ePjwjQTJdPIP/ZlgMqELnLF8rlTnnkP89b3Jd4iPo4Q/AzENFY98oZ3LMQea7Sby/WIJp4sJA4H4DuFPbke2rpNyr+6zdzLpFLI/ApyPLE14Szrcd4gpFR6yrS+4p4LpomNLr8YL7YLc9kgqebQcfYc47ZU89btJgpkc7D7nL+HzdzCqbb3zdzxyoD7y9zILbbFnpWFyBFEGS4LpApazp4f8fW6yrQc8fpG8/YBaf8OPaVFaFMNn0474B8z+rT/popBNFL94fIlLBlcaFYPyLGUHjIj2ecjwjHjKc=="
	xt := "1775912333650"
	xsCommon := "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"
	traceID := "ea7baeb1f2c9df30"
	xrayID := "cebe4fd828e702d46f9727dbb8cf4fcc"

	// Cookie 字符串
	cookieStr := "abRequestId=a44b4a68-7f83-5892-b43a-17b5cb20c2dc; ets=1775912013114; webBuild=6.5.1; xsecappid=xhs-pc-web; a1=19d7c9acdc9ipo6i2etzuudsx6w3pljhhiz4g12b250000236488; webId=46a1a02330a2601666cdccc4be1b515a; acw_tc=0a0bb4a517759120141864064e4ad73d33f3a44e0d072ad69fff77d3c2cb35; gid=yjfWSj0dq8xyyjfWSj0Sf3yJSj3UlK3Jd9u7uDJ7fMCKE428qU1Vxk888JqK4YY842di8Y80; web_session=040069b37d6085d4171a050cef3b4b7b2d5a00; id_token=VjEAAO4AA6Ic73WH93FppnUl6Wl8HV3FP95VEUQ3FvR4r2iuqCv6myjH9B8gAwO8ihVajRhjiz2mOHU7bmrSSnvcEsTV0ucYbVgbL4HcEuBFOQT6y7svu89zLrYhEwLSYDZxl0Kx; unread=%7B%22ub%22%3A%2269d86b1f000000001a0209a9%22%2C%22ue%22%3A%2269d87462000000001f0075be%22%2C%22uc%22%3A16%7D; websectiga=634d3ad75ffb42a2ade2c5e1705a73c845837578aeb31ba0e442d75c648da36a; sec_poison_id=b409086f-8f7c-4bb2-9d1e-04ed8358965a; loadts=1775912332830"

	// 测试 1: GET /folder/list（使用 curl 中的签名头）
	fmt.Println("\n1. GET /folder/list (使用签名头)...")
	req1, _ := http.NewRequest("GET", "https://edith.xiaohongshu.com/api/sns/web/v1/folder/list", nil)
	req1.Header.Set("Cookie", cookieStr)
	req1.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req1.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req1.Header.Set("Origin", "https://www.xiaohongshu.com")
	req1.Header.Set("X-s", xs)
	req1.Header.Set("X-t", xt)
	req1.Header.Set("X-S-Common", xsCommon)
	req1.Header.Set("X-B3-Traceid", traceID)
	req1.Header.Set("X-Xray-Traceid", xrayID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp1, err := client.Do(req1)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
	} else {
		defer resp1.Body.Close()
		body1, _ := io.ReadAll(resp1.Body)
		fmt.Printf("Status: %d\n", resp1.StatusCode)
		fmt.Printf("Body: %s\n", string(body1)[:min(500, len(body1))])
	}

	// 测试 2: POST /folder（创建专辑，使用签名头）
	fmt.Println("\n2. POST /folder (创建专辑，使用签名头)...")
	body := map[string]interface{}{
		"name": "TestAlbum_FromCurl",
		"type": "collect",
	}
	bodyBytes, _ := json.Marshal(body)
	req2, _ := http.NewRequest("POST", "https://edith.xiaohongshu.com/api/sns/web/v1/folder", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req2.Header.Set("Cookie", cookieStr)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req2.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req2.Header.Set("Origin", "https://www.xiaohongshu.com")
	req2.Header.Set("X-s", xs)
	req2.Header.Set("X-t", xt)
	req2.Header.Set("X-S-Common", xsCommon)
	req2.Header.Set("X-B3-Traceid", traceID)
	req2.Header.Set("X-Xray-Traceid", xrayID)

	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
	} else {
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)
		fmt.Printf("Status: %d\n", resp2.StatusCode)
		fmt.Printf("Body: %s\n", string(body2)[:min(500, len(body2))])
	}

	// 测试 3: 使用当前时间戳重新生成签名头（可能需要动态签名）
	fmt.Println("\n3. POST /folder (使用新时间戳)...")
	newTimestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	req3, _ := http.NewRequest("POST", "https://edith.xiaohongshu.com/api/sns/web/v1/folder", bytes.NewBuffer(bodyBytes))
	req3.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req3.Header.Set("Cookie", cookieStr)
	req3.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req3.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req3.Header.Set("Origin", "https://www.xiaohongshu.com")
	req3.Header.Set("X-s", xs)
	req3.Header.Set("X-t", newTimestamp) // 使用新时间戳
	req3.Header.Set("X-S-Common", xsCommon)
	req3.Header.Set("X-B3-Traceid", traceID)
	req3.Header.Set("X-Xray-Traceid", xrayID)

	resp3, err := client.Do(req3)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
	} else {
		defer resp3.Body.Close()
		body3, _ := io.ReadAll(resp3.Body)
		fmt.Printf("Status: %d\n", resp3.StatusCode)
		fmt.Printf("Body: %s\n", string(body3)[:min(500, len(body3))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
