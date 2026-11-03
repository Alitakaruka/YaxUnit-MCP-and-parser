package YaxUnitParser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
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

type PageData struct {
	URL    string
	Header string
	Data   string
}

func ParseAll(handleFunc func(Data PageData)) {
	Parser := newMainParser()
	go func() {
		for data := range Parser.Pages {
			handleFunc(data)
		}
	}()
	Parser.startParse()
	Parser.Close()
}

type mainParser struct {
	Pages   chan PageData
	wg      sync.WaitGroup
	client  *http.Client
	visited map[string]struct{}
	mu      sync.Mutex
}

func newMainParser() *mainParser {
	mp := mainParser{}

	mp.Pages = make(chan PageData, 10)

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
	pagePath, _ = strings.CutSuffix(pagePath, "/")
	base := path.Base(pagePath)
	parts := path.Dir(pagePath)

	return parts + "/" + url.PathEscape(base)
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

func (P *mainParser) parsePage(html []byte) (string, PageData) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		log.Println(err)
		return "", PageData{}
	}

	nextUrl, _ := doc.Find(".pagination-nav__link.pagination-nav__link--next").Attr("href")

	pageText := doc.Find(".theme-doc-markdown").Text()
	header := ""
	doc.Find("header").Each(func(i int, s *goquery.Selection) {
		header = s.Find("h1").Text()
	})

	Result := PageData{Header: header, Data: pageText}
	return nextUrl, Result
}
