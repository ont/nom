package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

type Parser struct {
	pagesConfigs  map[string]*ConfigEntity
	blocksConfigs map[string]*ConfigEntity

	logist *Logist

	errors chan *Page
	queue  chan *Page

	processed map[string]bool // set of urls (map keys) which already was processed (avoiding pages with self-references circular references and similar)
}

func NewParser(config *Grammar, logist *Logist) *Parser {
	parser := &Parser{
		pagesConfigs:  make(map[string]*ConfigEntity),
		blocksConfigs: make(map[string]*ConfigEntity),

		logist: logist,

		errors: make(chan *Page),
		queue:  make(chan *Page, 100000),

		processed: make(map[string]bool),
	}

	for _, entity := range config.Entities {
		switch entity.Type {
		case "page":
			parser.pagesConfigs[entity.Name] = entity
		case "block":
			parser.blocksConfigs[entity.Name] = entity
		}
	}

	return parser
}

func (p *Parser) Start() {
	logrus.Info("parser: starting")

	go p.processErrors()  // process parsing errors and errors from logist
	go p.processLogist()  // take pages from logist and parse them
	go p.processQueue()   // send from queue to logist (this goroutine is for avoiding deadlocks)
	go p.printQueueInfo() // prints statistic every N seconds
	go p.logist.Start()   // start logist
}

func (p *Parser) processErrors() {
	inform := func(page *Page) {
		logrus.WithField("url", page.Url).WithError(page.err).Error("parser: rejected [DROPPED TO NOWHERE]")
	}
	for {
		select {
		case page := <-p.logist.Errors():
			inform(page)
		case page := <-p.errors:
			inform(page)
		}
	}
}

func (p *Parser) processLogist() {
	for page := range p.logist.Delivery() {
		// don't parse files
		if !page.IsFile {
			p.parse(page)
		}

		p.logist.Store(page)
	}
}

func (p *Parser) processQueue() {
	for page := range p.queue {
		if p.processed[page.Url] {
			logrus.WithField("url", page.Url).Printf("parser: skip page (already processed)")
			continue
		}

		logrus.WithField("url", page.Url).Printf("parser: sending to logist")
		p.logist.Fetch(page)

		p.processed[page.Url] = true
	}
}

func (p *Parser) printQueueInfo() {
	for {
		logrus.WithField("items", len(p.queue)).Info("parser: queue info")
		time.Sleep(10 * time.Second)
	}
}

func (p *Parser) Queue(page *Page) {
	p.queue <- page
}

func (p *Parser) parse(page *Page) {
	logrus.WithField("url", page.Url).Info("parser: parsing fetched page")
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(page.Body))

	if err != nil {
		p.dropPage(page, err)
		return
	}

	config := p.pagesConfigs[page.Name]
	if config == nil {
		p.dropPage(page, fmt.Errorf("unknown page type \"%s\"", page.Name))
		return
	}

	//     ___ save parsed results here
	//    /                          ___ what to parse
	//   /                          /              ___ how to parse
	//  /                          /              /
	page.Tree = p.parseRecursive(doc.Selection, config)

	//spew.Dump(page.Tree)
}

func (p *Parser) parseRecursive(doc *goquery.Selection, config *ConfigEntity) *Block {
	// prepare place for savement
	block := &Block{
		Fields: make(map[string][]ValueOrBlock),
	}

	for _, route := range config.Routes {
		sel := doc.Find(route.Selector) // sub-document selection

		var data []ValueOrBlock
		// TODO: mediator pattern?
		switch route.Type {
		case "page":
			data = p.parsePages(sel, route)
		case "block":
			data = p.parseBlocks(sel, route)
		case "file":
			data = p.downloadPages(sel, route)
		}

		block.Fields[route.Name] = data
	}

	return block
}

func (p *Parser) parsePages(sel *goquery.Selection, route *Route) []ValueOrBlock {
	logrus.WithField("name", route.Name).Info("parser: parsing pages")

	res := make([]ValueOrBlock, 0)
	for _, url := range p.extractUrls(sel) {
		logrus.WithField("url", url).Info("parser: found new page")

		res = append(res, url)

		p.Queue(&Page{
			Name: route.Name,
			Url:  url,
		})
	}

	return res
}

func (p *Parser) parseBlocks(sel *goquery.Selection, route *Route) []ValueOrBlock {
	logrus.WithField("name", route.Name).Info("parser: parsing blocks")
	return nil
}

func (p *Parser) downloadPages(sel *goquery.Selection, route *Route) []ValueOrBlock {
	logrus.WithField("name", route.Name).Info("parser: parsing downloads")

	res := make([]ValueOrBlock, 0)

	for _, url := range p.extractUrls(sel) {
		logrus.WithField("url", url).Info("parser: found new download")

		res = append(res, url)

		p.Queue(&Page{
			Name:   route.Name,
			Url:    url,
			IsFile: true,
		})
	}

	return res
}

func (p *Parser) extractUrls(sel *goquery.Selection) []string {
	logrus.WithField("count", len(sel.Nodes)).Info("parser: extract urls from selector")

	urls := make([]string, 0, len(sel.Nodes))

	sel.Each(func(idx int, sel *goquery.Selection) {
		var url, found = "", false

		switch {
		case sel.Find("[href]").First().Length() > 0:
			url, found = sel.Find("[href]").First().Attr("href")

		case sel.Closest("[href]").Length() > 0:
			url, found = sel.Closest("[href]").Attr("href")
		}

		if found {
			urls = append(urls, url)
		}
	})

	return urls
}

func (p *Parser) dropPage(page *Page, err error) {
	page.err = err
	p.errors <- page
}
