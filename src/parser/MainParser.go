package YaxUnitParser

import (
	"YaxUnitMCP/service"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	urlTemplate = "https://bia-technologies.github.io%s"

	DescriptionFirstPage = "/yaxunit/docs/getting-started"
	ApiFirsPage          = "/yaxunit/api/Мокито"
)

func ParseAll(handleFunc func(Data service.PageData)) {
	Parser := newMainParser()
	go func() {
		Parser.startParse()
		Parser.Close()
	}()
	for data := range Parser.Pages {
		handleFunc(data)
	}
}

type mainParser struct {
	Pages   chan service.PageData
	wg      sync.WaitGroup
	client  *http.Client
	visited map[string]struct{}
	mu      sync.Mutex
}

func newMainParser() *mainParser {
	mp := mainParser{}

	mp.Pages = make(chan service.PageData, 10)

	mp.client = &http.Client{}
	mp.client.Timeout = time.Second * 5
	mp.visited = make(map[string]struct{})
	return &mp
}

func (P *mainParser) Close() {
	P.wg.Wait()
	close(P.Pages)
}

func (P *mainParser) startParse() {
	P.wg.Go(func() {
		P.processAll(DescriptionFirstPage)
	})
	P.wg.Go(func() {
		P.processAll(ApiFirsPage)
	})
}

func (P *mainParser) processAll(pagePath string) {

	pagePath = GetNormalPath(pagePath)
	url := fmt.Sprintf(urlTemplate, pagePath)
	page, err := P.getPage(url)
	if err != nil {
		log.Println(err)
		return
	}
	next, parseData := P.parsePage(page)
	if next != "" {
		P.wg.Go(func() {
			P.processAll(next)
		})
	}

	if parseData.Header == "" && parseData.Data == "" {
		log.Printf("Page:%s is empty!", url)
		return
	}

	parseData.URL = url
	P.Pages <- parseData
}

func GetNormalPath(pagePath string) string {

	parsed, err := url.Parse(pagePath)
	if err != nil {
		return pagePath
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = strings.TrimSuffix(parsed.RawPath, "/")
	return parsed.EscapedPath()
}

func (P *mainParser) getPage(url string) ([]byte, error) {

	if P.markVisited(url) {
		return []byte{}, nil
	}

	resp, err := P.client.Get(url)

	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, errors.New("Error of geting page! Status:" + strconv.Itoa(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}

func (P *mainParser) markVisited(url string) bool {
	P.mu.Lock()
	defer P.mu.Unlock()

	if _, exists := P.visited[url]; exists {
		return true
	}

	P.visited[url] = struct{}{}
	return false
}

func (P *mainParser) parsePage(html []byte) (string, service.PageData) {

	Result := service.PageData{}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		log.Println(err)
		return "", service.PageData{}
	}

	nextUrl, _ := doc.Find(".pagination-nav__link.pagination-nav__link--next").Attr("href")

	module, err := parseHTML(html)

	if err == nil {
		Result.Module = *module
	}

	pageText := doc.Find(".theme-doc-markdown").Text()
	header := ""
	doc.Find("header").Each(func(i int, s *goquery.Selection) {
		header = s.Find("h1").Text()
	})

	Result.Data = pageText
	Result.Header = header
	Result.URL = ""
	return nextUrl, Result
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func parseHTML(html []byte) (*service.Module, error) {
	doc, err := goquery.NewDocumentFromReader(
		(bytes.NewReader(html)),
	)
	if err != nil {
		return nil, err
	}

	module := &service.Module{
		Name: cleanText(doc.Find("h1").First().Text()),
	}

	doc.Find("article h3").Each(func(_ int, heading *goquery.Selection) {
		method := service.Method{
			Name: strings.TrimSpace(heading.Find("code").Text()),
		}

		if method.Name == "" {
			return
		}

		var section []*goquery.Selection

		for node := heading.Next(); node.Length() > 0; node = node.Next() {
			if node.Is("h3") {
				break
			}
			section = append(section, node)
		}

		var description []string

		for _, node := range section {
			if node.Is("p") && node.Find("strong").Length() != 1 {
				text := cleanText(node.Text())
				if text != "" {
					description = append(description, text)
				}
			}

			if node.Find("strong").FilterFunction(func(_ int, s *goquery.Selection) bool {
				return cleanText(s.Text()) == "Параметры метода"
			}).Length() > 0 {
				method.Parameters = parseParameters(node.Next())
			}

			if node.Find("strong").FilterFunction(func(_ int, s *goquery.Selection) bool {
				return cleanText(s.Text()) == "Возвращает"
			}).Length() > 0 {
				method.Returns = parseReturn(node)
			}

			// Извлекаем примеры кода.
			node.Find("pre code").Each(func(_ int, code *goquery.Selection) {
				example := strings.TrimSpace(code.Text())
				if example != "" {
					method.Examples = append(method.Examples, example)
				}
			})
		}

		method.Description = strings.Join(description, " ")

		module.Methods = append(module.Methods, method)
	})

	// js, _ := json.Marshal(module)
	// fmt.Printf("js: %v\n", string(js))
	return module, nil
}

func parseParameters(section *goquery.Selection) []service.Parameter {
	var result []service.Parameter

	section.ChildrenFiltered("li").Each(func(_ int, li *goquery.Selection) {

		param := service.Parameter{
			Name: cleanText(li.ChildrenFiltered("code").First().Text()),
		}

		if param.Name == "" {
			return
		}

		firstNested := li.ChildrenFiltered("ul").First()

		firstNested.ChildrenFiltered("li").Each(func(_ int, item *goquery.Selection) {
			code := item.ChildrenFiltered("code")

			if code.Length() > 0 {
				param.Type = cleanText(code.First().Text())
			}

			MoreInformation := item.ChildrenFiltered("ul")

			text := cleanText(MoreInformation.Clone().Remove().End().Text())

			if text != "" {
				param.Description = strings.TrimSpace(
					strings.TrimPrefix(text, param.Type+" -"),
				)
			}

			MoreInformation.ChildrenFiltered("li").Each(func(_ int, child *goquery.Selection) {
				param.Values = append(param.Values, service.Parameter{
					Name: cleanText(child.Find("code").First().Text()),
					Description: cleanText(
						getContentText(child),
					),
				})
			})
		})

		result = append(result, param)
	})

	return result
}

func getContentText(child *goquery.Selection) string {
	var result string
	child.Contents().Each(func(i int, s *goquery.Selection) {
		if goquery.NodeName(s) == "#text" {
			result += s.Text()
		}
	})
	return cleanText(result)
}
func parseReturn(section *goquery.Selection) *service.Return {
	p := section.Find("p").First()

	if p.Length() == 0 {
		return nil
	}

	return &service.Return{
		Type:        cleanText(p.Find("code").First().Text()),
		Description: cleanText(p.Clone().Find("code").Remove().End().Text()),
	}
}
