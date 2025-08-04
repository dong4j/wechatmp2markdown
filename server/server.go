package server

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/fengxxc/wechatmp2markdown/format"
	"github.com/fengxxc/wechatmp2markdown/parse"
	"github.com/fengxxc/wechatmp2markdown/util"
)

func Start(addr string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rawQuery := r.URL.RawQuery
		paramsMap := parseParams(rawQuery)

		// url param
		wechatmpURL := paramsMap["url"]
		fmt.Printf("accept url: %s\n", wechatmpURL)
		imageArgValue := paramsMap["image"]
		fmt.Printf("     image: %s\n", imageArgValue)
		proxy := paramsMap["proxy"]
		fmt.Printf("     proxy: %s\n", proxy)
		imagePolicy := parse.ImageArgValue2ImagePolicy(imageArgValue)

		if wechatmpURL == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(defHTML))
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		var articleStruct parse.Article
		if proxy != "" {
			// 尝试使用代理
			defer func() {
				if r := recover(); r != nil {
					log.Printf("代理请求失败，尝试不使用代理: %v", r)
					// 如果代理失败，降级到不使用代理
					articleStruct = parse.ParseFromURL(wechatmpURL, imagePolicy)
				}
			}()
			articleStruct = parse.ParseFromURLWithProxy(wechatmpURL, imagePolicy, proxy)
		} else {
			articleStruct = parse.ParseFromURL(wechatmpURL, imagePolicy)
		}
		title := articleStruct.Title.Val.(string)
		mdString, saveImageBytes := format.Format(articleStruct)
		if len(saveImageBytes) > 0 {
			w.Header().Set("Content-Disposition", "attachment; filename="+title+".zip")
			saveImageBytes[title] = []byte(mdString)
			util.HttpDownloadZip(w, saveImageBytes)
		} else {
			w.Header().Set("Content-Disposition", "attachment; filename="+title+".md")
			w.Write([]byte(mdString))
		}
	})

	fmt.Printf("wechatmp2markdown server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

var defHTML string = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>微信公众号文章转Markdown工具</title>
	<style>
		* {
			margin: 0;
			padding: 0;
			box-sizing: border-box;
		}
		
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
			background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			padding: 20px;
		}
		
		.container {
			background: rgba(255, 255, 255, 0.95);
			border-radius: 20px;
			box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
			padding: 40px;
			max-width: 800px;
			width: 100%;
			backdrop-filter: blur(10px);
		}
		
		.header {
			text-align: center;
			margin-bottom: 40px;
		}
		
		.logo {
			font-size: 2.5em;
			font-weight: 700;
			background: linear-gradient(45deg, #667eea, #764ba2);
			-webkit-background-clip: text;
			-webkit-text-fill-color: transparent;
			background-clip: text;
			margin-bottom: 10px;
		}
		
		.subtitle {
			color: #666;
			font-size: 1.1em;
			font-weight: 300;
		}
		
		.usage-section {
			margin-bottom: 30px;
		}
		
		.section-title {
			font-size: 1.3em;
			font-weight: 600;
			color: #333;
			margin-bottom: 20px;
			display: flex;
			align-items: center;
		}
		
		.section-title::before {
			content: "📖";
			margin-right: 10px;
			font-size: 1.2em;
		}
		
		.param-list {
			list-style: none;
			background: #f8f9fa;
			border-radius: 12px;
			padding: 20px;
			margin-bottom: 20px;
		}
		
		.param-item {
			margin-bottom: 15px;
			padding: 15px;
			background: white;
			border-radius: 8px;
			border-left: 4px solid #667eea;
			box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
		}
		
		.param-item:last-child {
			margin-bottom: 0;
		}
		
		.param-name {
			font-weight: 600;
			color: #667eea;
			display: inline-block;
			margin-bottom: 5px;
		}
		
		.param-desc {
			color: #555;
			line-height: 1.5;
		}
		
		.example-section {
			background: #e3f2fd;
			border-radius: 12px;
			padding: 20px;
			margin-top: 20px;
		}
		
		.example-title {
			font-size: 1.2em;
			font-weight: 600;
			color: #1976d2;
			margin-bottom: 15px;
			display: flex;
			align-items: center;
		}
		
		.example-title::before {
			content: "💡";
			margin-right: 10px;
		}
		
		.example-url {
			background: #fff;
			padding: 15px;
			border-radius: 8px;
			font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
			font-size: 0.9em;
			color: #333;
			word-break: break-all;
			border: 1px solid #e0e0e0;
			box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
		}
		
		.features {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
			gap: 20px;
			margin-top: 30px;
		}
		
		.feature-item {
			text-align: center;
			padding: 20px;
			background: white;
			border-radius: 12px;
			box-shadow: 0 4px 8px rgba(0, 0, 0, 0.05);
			transition: transform 0.2s ease;
		}
		
		.feature-item:hover {
			transform: translateY(-2px);
		}
		
		.feature-icon {
			font-size: 2em;
			margin-bottom: 10px;
		}
		
		.feature-title {
			font-weight: 600;
			color: #333;
			margin-bottom: 5px;
		}
		
		.feature-desc {
			color: #666;
			font-size: 0.9em;
		}
		
		@media (max-width: 768px) {
			.container {
				padding: 20px;
			}
			
			.logo {
				font-size: 2em;
			}
			
			.features {
				grid-template-columns: 1fr;
			}
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<div class="logo">wechatmp2markdown</div>
			<div class="subtitle">微信公众号文章转Markdown工具</div>
		</div>
		
		<div class="usage-section">
			<div class="section-title">使用说明</div>
			<div class="param-list">
				<div class="param-item">
					<div class="param-name">url 参数（必需）</div>
					<div class="param-desc">请输入微信公众号文章的URL地址</div>
				</div>
				<div class="param-item">
					<div class="param-name">image 参数（可选）</div>
					<div class="param-desc">图片保存方式：'url'（引用原地址） / 'save'（保存到本地） / 'base64'（编码到文件内，默认）</div>
				</div>
				<div class="param-item">
					<div class="param-name">proxy 参数（可选）</div>
					<div class="param-desc">代理服务器地址，格式：'ip:port'，例如：'127.0.0.1:8080'</div>
				</div>
			</div>
			
			<div class="example-section">
				<div class="example-title">使用示例</div>
				<div class="example-url">http://localhost:8964/?url=https://mp.weixin.qq.com/s?__biz=aaaa==&mid=1111&idx=2&sn=bbbb&chksm=cccc&scene=123&image=save&proxy=127.0.0.1:8080</div>
			</div>
		</div>
		
		<div class="features">
			<div class="feature-item">
				<div class="feature-icon">📝</div>
				<div class="feature-title">Markdown转换</div>
				<div class="feature-desc">将微信公众号文章转换为标准Markdown格式</div>
			</div>
			<div class="feature-item">
				<div class="feature-icon">🖼️</div>
				<div class="feature-title">图片处理</div>
				<div class="feature-desc">支持多种图片保存方式，包括本地保存和Base64编码</div>
			</div>
			<div class="feature-item">
				<div class="feature-icon">🌐</div>
				<div class="feature-title">代理支持</div>
				<div class="feature-desc">支持代理服务器，提高访问成功率</div>
			</div>
			<div class="feature-item">
				<div class="feature-icon">⚡</div>
				<div class="feature-title">快速转换</div>
				<div class="feature-desc">高效解析，快速生成Markdown文件</div>
			</div>
		</div>
	</div>
</body>
</html>
`

func parseParams(rawQuery string) map[string]string {
	result := make(map[string]string)

	// 解析 image 参数
	reg := regexp.MustCompile(`(&?image=)([a-z]+)`)
	matcheImage := reg.FindStringSubmatch(rawQuery)
	var remainingQuery string = rawQuery
	if len(matcheImage) > 1 {
		// 有image参数
		imageParamFull := matcheImage[0]
		remainingQuery = strings.Replace(rawQuery, imageParamFull, "", 1)

		if len(matcheImage) > 2 {
			imageParamVal := matcheImage[2]
			result["image"] = imageParamVal
		}
	}

	// 解析 proxy 参数
	regProxy := regexp.MustCompile(`(&?proxy=)([^&]+)`)
	matcheProxy := regProxy.FindStringSubmatch(remainingQuery)
	if len(matcheProxy) > 1 {
		// 有proxy参数
		proxyParamFull := matcheProxy[0]
		remainingQuery = strings.Replace(remainingQuery, proxyParamFull, "", 1)

		if len(matcheProxy) > 2 {
			proxyParamVal := matcheProxy[2]
			result["proxy"] = proxyParamVal
		}
	}

	// 解析 url 参数
	regUrl := regexp.MustCompile(`(&?url=)(.+)`)
	matcheUrl := regUrl.FindStringSubmatch(remainingQuery)
	if len(matcheUrl) > 2 {
		urlParamVal := matcheUrl[2]
		result["url"] = urlParamVal
	}
	return result
}
