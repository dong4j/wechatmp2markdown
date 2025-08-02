package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

func main() {

	// 代理服务器
	proxy_raw := "119.101.104.230:34849"
	proxy_str := fmt.Sprintf("http://%s", proxy_raw)
	proxy, err := url.Parse(proxy_str)

	// 目标网页
	page_url := "https://mp.weixin.qq.com/s/JaYq2ZXpd4UF9bFdQzPQ3Q"

	//  请求目标网页
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	req, _ := http.NewRequest("GET", page_url, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36") 
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("Cache-Control", "max-age=0")
	req.Header.Add("Sec-Fetch-Dest", "document")
	req.Header.Add("Sec-Fetch-Mode", "navigate")
	req.Header.Add("Sec-Fetch-Site", "none")
	req.Header.Add("Sec-Fetch-User", "?1")
	res, err := client.Do(req)

	if err != nil {
 		// 请求发生异常
		fmt.Println(err.Error())
	} else {
		defer res.Body.Close() //保证最后关闭Body

		fmt.Println("status code:", res.StatusCode) // 获取状态码

		// 有gzip压缩时,需要解压缩读取返回内容
		if res.Header.Get("Content-Encoding") == "gzip" {
			reader, _ := gzip.NewReader(res.Body) // gzip解压缩
			defer reader.Close()
			io.Copy(os.Stdout, reader)
			return // 正常退出
		}

		// 无gzip压缩, 读取返回内容
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
	}
}