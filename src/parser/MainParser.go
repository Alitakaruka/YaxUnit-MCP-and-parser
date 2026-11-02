package YaxUnitParser

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

const (
	urlTemplate = "https://bia-technologies.github.io%s"

	DescriptionFirstPage = "/yaxunit/docs/getting-started"
	ApiFirsPage          = "/yaxunit/api/Мокито"
)

type PageData struct {
	Header string
	Data   string
}

func ParseAll(handeFunc func(Data PageData)) {
	Parser := newMainParser()
	go func() {
		for data := range Parser.Pages {
			handeFunc(data)
		}
	}()
	Parser.startParse()
	Parser.Wait()
	close(Parser.Pages)
}

type mainParser struct {
	Pages chan PageData
	WG    sync.WaitGroup
}

func newMainParser() *mainParser {
	MP := mainParser{}
	MP.Pages = make(chan PageData, 10)
	return &MP
}

func (P *mainParser) Wait() {
	P.WG.Wait()
}

func (P *mainParser) startParse() {
	P.WG.Add(2)
	go P.processAll(DescriptionFirstPage)
	go P.processAll(ApiFirsPage)
}

func (P *mainParser) processAll(PagePath string) {

	PagePath = GetNormalPath(PagePath)
	url := fmt.Sprintf(urlTemplate, PagePath)
	page, err := P.getPage(url)
	if err != nil {
		log.Println(err)
		return
	}
	Next, ParseData := P.parsePage(page)
	if Next != "" {
		P.WG.Go(func() {
			P.processAll(Next)
		})
	}

	if ParseData.Header == "" && ParseData.Data == "" {
		log.Printf("Page:%s is empty!", url)
		return
	}

	P.Pages <- ParseData
}

func GetNormalPath(PagePath string) string {
	PagePath, _ = strings.CutSuffix(PagePath, "/")
	Base := path.Base(PagePath)
	Parts := path.Dir(PagePath)

	return Parts + "/" + url.PathEscape(Base)
}

func (P *mainParser) getPage(url string) ([]byte, error) {
	fmt.Printf("url: %v\n", url)
	resp, err := http.Get(url)

	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}

func (P *mainParser) parsePage(html []byte) (string, PageData) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		log.Println(err)
	}

	NextUrl, _ := doc.Find(".pagination-nav__link.pagination-nav__link--next").Attr("href")

	pageText := doc.Find(".theme-doc-markdown").Text()
	Header := ""
	doc.Find("header").Each(func(i int, s *goquery.Selection) {
		Header = s.Find("h1").Text()
	})

	Result := PageData{Header: Header, Data: pageText}
	return NextUrl, Result
}
