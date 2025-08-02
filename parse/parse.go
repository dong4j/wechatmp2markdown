package parse

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func parseSection(s *goquery.Selection, imagePolicy ImagePolicy, lastPieceType PieceType) []Piece {
	return parseSectionWithProxy(s, imagePolicy, lastPieceType, "")
}

func parseSectionWithProxy(s *goquery.Selection, imagePolicy ImagePolicy, lastPieceType PieceType, proxy string) []Piece {
	var pieces []Piece
	if lastPieceType == O_LIST || lastPieceType == U_LIST || lastPieceType == NULL || lastPieceType == BLOCK_QUOTES {
		// pieces = append(pieces, Piece{NULL, nil, nil})
	} else {
		pieces = append(pieces, Piece{BR, nil, nil})
	}
	var _lastPieceType PieceType = NULL
	s.Contents().Each(func(i int, sc *goquery.Selection) {
		attr := make(map[string]string)
		if sc.Is("a") {
			attr["href"], _ = sc.Attr("href")
			pieces = append(pieces, Piece{LINK, removeBrAndBlank(sc.Text()), attr})
		} else if sc.Is("img") {
			// 优化图片处理，支持微信公众号的图片格式
			src, _ := sc.Attr("data-src")
			if src == "" {
				src, _ = sc.Attr("src")
			}
			attr["src"] = src
			attr["alt"], _ = sc.Attr("alt")
			attr["title"], _ = sc.Attr("title")
			
			// 处理微信公众号的图片水印和格式
			if strings.Contains(src, "mmbiz.qpic.cn") {
				// 移除微信图片的压缩参数，获取原图
				if strings.Contains(src, "wx_fmt=") {
					src = strings.Split(src, "&wx_fmt=")[0] + "&wx_fmt=jpeg"
				}
				attr["src"] = src
			}
			
			switch imagePolicy {
			case IMAGE_POLICY_URL:
				pieces = append(pieces, Piece{IMAGE, nil, attr})
			case IMAGE_POLICY_SAVE:
				image := fetchImgFileWithProxy(attr["src"], proxy)
				pieces = append(pieces, Piece{IMAGE, image, attr})
			case IMAGE_POLICY_BASE64:
				fallthrough
			default:
				base64Image := img2base64(fetchImgFileWithProxy(attr["src"], proxy))
				pieces = append(pieces, Piece{IMAGE_BASE64, base64Image, attr})
			}
		} else if sc.Is("ol") {
			pieces = append(pieces, parseListWithProxy(sc, O_LIST, imagePolicy, proxy)...)
		} else if sc.Is("ul") {
			pieces = append(pieces, parseListWithProxy(sc, U_LIST, imagePolicy, proxy)...)
		} else if sc.Is("pre") || sc.Is("section.code-snippet__fix") || sc.Is("code") {
			// 代码块
			pieces = append(pieces, parsePre(sc)...)
		} else if sc.Is("span") || sc.Is("figure") {
			pieces = append(pieces, parseSectionWithProxy(sc, imagePolicy, _lastPieceType, proxy)...)
		} else if sc.Is("p") || sc.Is("section") || sc.Is("figcaption") {
			pieces = append(pieces, parseSectionWithProxy(sc, imagePolicy, _lastPieceType, proxy)...)
			if removeBrAndBlank(sc.Text()) != "" && len(pieces) > 0 && pieces[len(pieces)-1].Type != BR {
				pieces = append(pieces, Piece{BR, nil, nil})
			}
		} else if sc.Is("h1") || sc.Is("h2") || sc.Is("h3") || sc.Is("h4") || sc.Is("h5") || sc.Is("h6") {
			pieces = append(pieces, parseHeader(sc)...)
		} else if sc.Is("blockquote") {
			pieces = append(pieces, parseBlockQuoteWithProxy(sc, imagePolicy, proxy)...)
		} else if sc.Is("strong") || sc.Is("b") {
			pieces = append(pieces, parseStrong(sc)...)
		} else if sc.Is("em") || sc.Is("i") {
			// 处理斜体文本
			pieces = append(pieces, Piece{ITALIC_TEXT, removeBrAndBlank(sc.Text()), nil})
		} else if sc.Is("table") {
			pieces = append(pieces, parseTable(sc)...)
		} else if sc.Is("hr") {
			// 处理分隔线
			pieces = append(pieces, Piece{HR, nil, nil})
		} else if sc.Is("br") {
			// 处理换行
			pieces = append(pieces, Piece{BR, nil, nil})
		} else {
			// 处理微信公众号特有的元素
			if sc.Is("mpvoice") || sc.Is("mp-common-mpaudio") {
				// 处理语音消息
				voiceId, _ := sc.Attr("voice_encode_fileid")
				pieces = append(pieces, Piece{NORMAL_TEXT, "[语音消息: " + voiceId + "]", nil})
			} else if sc.Is("mpvideo") {
				// 处理视频消息
				videoId, _ := sc.Attr("vid")
				pieces = append(pieces, Piece{NORMAL_TEXT, "[视频消息: " + videoId + "]", nil})
			} else if sc.Is("qqmusic") || sc.Is("mp-common-qqmusic") {
				// 处理音乐消息
				musicName, _ := sc.Attr("data-name")
				pieces = append(pieces, Piece{NORMAL_TEXT, "[音乐: " + musicName + "]", nil})
			} else if sc.Is("mp-common-profile") {
				// 处理名片
				profileName, _ := sc.Attr("data-name")
				pieces = append(pieces, Piece{NORMAL_TEXT, "[名片: " + profileName + "]", nil})
			} else if sc.Is("mp-common-card") {
				// 处理卡片
				cardTitle, _ := sc.Attr("data-title")
				pieces = append(pieces, Piece{NORMAL_TEXT, "[卡片: " + cardTitle + "]", nil})
			} else if sc.Text() != "" {
				// 处理普通文本，优化空白字符处理
				text := removeBrAndBlank(sc.Text())
				if text != "" {
					pieces = append(pieces, Piece{NORMAL_TEXT, text, nil})
				}
			}
		}
		if len(pieces) > 0 {
			_lastPieceType = pieces[len(pieces)-1].Type
		}
	})
	return pieces
}

func parseHeader(s *goquery.Selection) []Piece {
	var level int
	switch {
	case s.Is("h1"):
		level = 1
	case s.Is("h2"):
		level = 2
	case s.Is("h3"):
		level = 3
	case s.Is("h4"):
		level = 4
	case s.Is("h5"):
		level = 5
	case s.Is("h6"):
		level = 6
	}
	attr := map[string]string{"level": strconv.Itoa(level)}
	p := Piece{HEADER, removeBrAndBlank(s.Text()), attr}
	return []Piece{p}
}

func parsePre(s *goquery.Selection) []Piece {
	// 优化代码块解析，支持更多代码格式
	var codeRows []string
	
	// 处理 <pre><code> 结构
	s.Find("code").Each(func(i int, sc *goquery.Selection) {
		var codeLine string = ""
		sc.Contents().Each(func(i int, sc *goquery.Selection) {
			if goquery.NodeName(sc) == "br" {
				codeRows = append(codeRows, codeLine)
				codeLine = ""
			} else {
				codeLine += sc.Text()
			}
		})
		codeRows = append(codeRows, codeLine)
	})
	
	// 如果没有找到 code 标签，直接处理 pre 标签内容
	if len(codeRows) == 0 {
		var codeLine string = ""
		s.Contents().Each(func(i int, sc *goquery.Selection) {
			if goquery.NodeName(sc) == "br" {
				codeRows = append(codeRows, codeLine)
				codeLine = ""
			} else {
				codeLine += sc.Text()
			}
		})
		codeRows = append(codeRows, codeLine)
	}
	
	// 清理空行
	var cleanedRows []string
	for _, row := range codeRows {
		if strings.TrimSpace(row) != "" {
			cleanedRows = append(cleanedRows, row)
		}
	}
	
	p := Piece{CODE_BLOCK, cleanedRows, nil}
	return []Piece{p}
}

func parseList(s *goquery.Selection, ptype PieceType, imagePolicy ImagePolicy) []Piece {
	return parseListWithProxy(s, ptype, imagePolicy, "")
}

func parseListWithProxy(s *goquery.Selection, ptype PieceType, imagePolicy ImagePolicy, proxy string) []Piece {
	var list []Piece
	s.Find("li").Each(func(i int, sc *goquery.Selection) {
		list = append(list, Piece{ptype, parseSectionWithProxy(sc, imagePolicy, ptype, proxy), nil})
	})
	return list
}

func parseBlockQuote(s *goquery.Selection, imagePolicy ImagePolicy) []Piece {
	return parseBlockQuoteWithProxy(s, imagePolicy, "")
}

func parseBlockQuoteWithProxy(s *goquery.Selection, imagePolicy ImagePolicy, proxy string) []Piece {
	var bq []Piece
	s.Contents().Each(func(i int, sc *goquery.Selection) {
		bq = append(bq, Piece{BLOCK_QUOTES, parseSectionWithProxy(sc, imagePolicy, BLOCK_QUOTES, proxy), nil})
	})
	bq = append(bq, Piece{BR, nil, nil})
	return bq
}

func parseTable(s *goquery.Selection) []Piece {
	// 优化表格解析，转换为 Markdown 格式
	var table []Piece
	
	// 尝试解析表格结构
	var rows []string
	s.Find("tr").Each(func(i int, tr *goquery.Selection) {
		var cells []string
		tr.Find("td, th").Each(func(j int, cell *goquery.Selection) {
			cellText := removeBrAndBlank(cell.Text())
			cells = append(cells, cellText)
		})
		if len(cells) > 0 {
			rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
		}
	})
	
	if len(rows) > 0 {
		// 添加表头分隔符
		if len(rows) > 1 {
			headerRow := rows[0]
			cellCount := strings.Count(headerRow, "|") - 1
			separator := "|"
			for i := 0; i < cellCount; i++ {
				separator += " --- |"
			}
			rows = append([]string{rows[0], separator}, rows[1:]...)
		}
		
		tableMd := strings.Join(rows, "\n")
		table = append(table, Piece{TABLE, tableMd, map[string]string{"type": "markdown"}})
	} else {
		// 如果无法解析，保留原始 HTML
		html, _ := s.Html()
		table = append(table, Piece{TABLE, "<table>" + html + "</table>", map[string]string{"type": "native"}})
	}
	
	return table
}

func parseStrong(s *goquery.Selection) []Piece {
	var bt []Piece
	bt = append(bt, Piece{BOLD_TEXT, strings.TrimSpace(s.Text()), nil})
	return bt
}

func parseMeta(s *goquery.Selection) []string {
	var res []string
	s.Children().Each(func(i int, sc *goquery.Selection) {
		if sc.Is("#profileBt") {
			authorName := removeBrAndBlank(sc.Find("#js_name").Text())
			if authorName != "" {
				res = append(res, authorName)
			}
		} else {
			style, exists := sc.Attr("style")
			if !(exists && strings.Contains(style, "display: none;")) {
				t := removeBrAndBlank(sc.Text())
				if t != "" {
					res = append(res, t)
				}
			}
		}
	})
	return res
}

func ParseFromReader(r io.Reader, imagePolicy ImagePolicy) Article {
	return ParseFromReaderWithProxy(r, imagePolicy, "")
}

func ParseFromReaderWithProxy(r io.Reader, imagePolicy ImagePolicy, proxy string) Article {
	var article Article
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		log.Fatal(err)
	}
	var mainContent *goquery.Selection = doc.Find("#img-content")

	// 标题
	title := mainContent.Find("#activity-name").Text()
	attr := map[string]string{"level": "1"}
	article.Title = Piece{HEADER, removeBrAndBlank(title), attr}

	// meta
	meta := mainContent.Find("#meta_content")
	metastring := parseMeta(meta)
	article.Meta = metastring
	// 从js中找到发布时间
	re, _ := regexp.Compile("var ct = \"([0-9]+)\"")
	findstrs := re.FindStringSubmatch(doc.Find("script").Text())
	if len(findstrs) > 1 {
		var createTime string = findstrs[1]
		timestamp, _ := strconv.Atoi(createTime)
		time := time.Unix(int64(timestamp), 0)
		article.Meta = append(article.Meta, time.Format("2006-01-02 15:04"))
	}

	// tags 细节待完善
	tags := mainContent.Find("#js_tags").Text()
	tags = removeBrAndBlank(tags)
	article.Tags = tags

	// content
	// section[style="line-height: 1.5em;"]>span,a	=> 一般段落（含文本和超链接）
	// p[style="line-height: 1.5em;"]				=> 项目列表（有序/无序）
	// section[style=".*text-align:center"]>img		=> 居中段落（图片）
	content := mainContent.Find("#js_content")
	pieces := parseSectionWithProxy(content, imagePolicy, NULL, proxy)
	article.Content = pieces

	return article
}

func ParseFromHTMLString(s string, imagePolicy ImagePolicy) Article {
	return ParseFromReader(strings.NewReader(s), imagePolicy)
}

func ParseFromHTMLFile(filepath string, imagePolicy ImagePolicy) Article {
	file, err := os.Open(filepath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	content, err2 := io.ReadAll(file)
	if err2 != nil {
		panic(err)
	}
	return ParseFromReader(bytes.NewReader(content), imagePolicy)
}

func ParseFromURL(url string, imagePolicy ImagePolicy) Article {
	return ParseFromURLWithProxy(url, imagePolicy, "")
}

func ParseFromURLWithProxy(targetURL string, imagePolicy ImagePolicy, proxy string) Article {
	req, err := http.NewRequest("GET", targetURL, nil)

	if err != nil {
		log.Printf("new request %s error: %s", targetURL, err.Error())
		return Article{} // 返回空结果而不是 panic
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0")
	
	// 设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// 如果提供了代理，设置代理
	if proxy != "" {
		proxyURL, err := url.Parse("http://" + proxy)
		if err != nil {
			log.Printf("invalid proxy format %s: %s", proxy, err.Error())
		} else {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				// 设置连接超时
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				// 设置空闲连接超时
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   30 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			}
		}
	}
	
	res, err := client.Do(req)
	if err != nil {
		log.Printf("request to url %s error: %s", targetURL, err.Error())
		return Article{} // 返回空结果而不是 panic
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("get from url %s error: %d %s", targetURL, res.StatusCode, res.Status)
		return Article{} // 返回空结果而不是 panic
	}
	return ParseFromReaderWithProxy(res.Body, imagePolicy, proxy)
}

func removeBrAndBlank(s string) string {
	// 优化文本清理，更好地处理微信公众号的文本格式
	s = strings.TrimSpace(s)
	
	// 移除多余的空白字符
	regstr := "\\s{2,}"
	reg, _ := regexp.Compile(regstr)
	s = reg.ReplaceAllString(s, " ")
	
	// 处理换行符
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	
	// 移除微信特有的空白字符
	s = strings.ReplaceAll(s, "\u00A0", " ") // 不间断空格
	s = strings.ReplaceAll(s, "\u200B", "")  // 零宽空格
	s = strings.ReplaceAll(s, "\u200C", "")  // 零宽非连接符
	s = strings.ReplaceAll(s, "\u200D", "")  // 零宽连接符
	
	// 再次清理多余空格
	s = reg.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	
	return s
}

func fetchImgFile(url string) []byte {
	return fetchImgFileWithProxy(url, "")
}

func fetchImgFileWithProxy(imgURL string, proxy string) []byte {
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		log.Printf("new request for image %s error: %s", imgURL, err.Error())
		return nil
	}
	
	// 设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// 如果提供了代理，设置代理
	if proxy != "" {
		proxyURL, err := url.Parse("http://" + proxy)
		if err != nil {
			log.Printf("invalid proxy format %s: %s", proxy, err.Error())
		} else {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				// 设置连接超时
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				// 设置空闲连接超时
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   30 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			}
		}
	}
	
	res, err := client.Do(req)
	if err != nil {
		log.Printf("get Image from url %s error: %s", imgURL, err.Error())
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("get Image from url %s error: %d %s", imgURL, res.StatusCode, res.Status)
		return nil
	}
	content, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("read image Response error: %s", err.Error())
		return nil
	}
	return content
}

func img2base64(content []byte) string {
	return base64.StdEncoding.EncodeToString(content)
}

type ImagePolicy int32

const (
	IMAGE_POLICY_URL ImagePolicy = iota
	IMAGE_POLICY_SAVE
	IMAGE_POLICY_BASE64
)

func ImageArgValue2ImagePolicy(val string) ImagePolicy {
	var imagePolicy ImagePolicy
	switch val {
	case "url":
		imagePolicy = IMAGE_POLICY_URL
	case "save":
		imagePolicy = IMAGE_POLICY_SAVE
	case "base64":
		fallthrough
	default:
		imagePolicy = IMAGE_POLICY_BASE64
	}
	return imagePolicy
}
