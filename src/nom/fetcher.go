package main

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/sirupsen/logrus"
)

type Fetcher interface {
	Errors() <-chan *Page
	Delivery() <-chan *Page
	Queue(page *Page)
	Start()
}

type FetcherSimple struct {
	Base  *url.URL
	Delay int

	delivery chan *Page
	errors   chan *Page
	queue    chan *Page
}

func NewFetcherSimple(base string, delay int) (*FetcherSimple, error) {
	baseUrl, err := url.Parse(base)
	if err != nil {
		return nil, err
	}

	return &FetcherSimple{
		Base:  baseUrl,
		Delay: delay,

		delivery: make(chan *Page),
		errors:   make(chan *Page),
		queue:    make(chan *Page),
	}, nil
}

// NOTE: Start this as goroutine
func (f *FetcherSimple) Start() {
	logrus.Info("fetcher: starting")

	// TODO: not optimal sleeping, we can interrupt sleep if it was longer than f.Delay
	for page := range f.queue {
		f.fetch(page) // process one page at time than go to sleep...
		time.Sleep(time.Duration(f.Delay) * time.Second)
	}
}

func (f *FetcherSimple) fetch(page *Page) {
	purl, err := url.Parse(page.Url)
	if err != nil {
		logrus.WithField("url", page.Url).WithError(err).Error("fetcher: wrong url")
		f.dropPage(page, err)
		return
	}

	page.FullUrl = f.Base.ResolveReference(purl).String()

	res, err := http.Get(page.FullUrl)
	if err != nil {
		f.dropPage(page, err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		f.dropPage(page, err)
		return
	}
	page.Body = body

	if res.StatusCode < 200 || res.StatusCode > 302 {
		f.dropPage(page, fmt.Errorf("Server returns error code %s", res.StatusCode))
		return
	}

	logrus.WithField("url", page.Url).Info("fetcher: success fetch")

	page.FinalUrl = res.Request.URL.String()

	if page.IsFile {
		err := f.parseFileName(res, page)
		if err != nil {
			f.dropPage(page, err)
			return
		}

		size := datasize.ByteSize(len(page.Body))
		logrus.WithField("file_name", page.FileName).WithField("final_url", page.FinalUrl).WithField("size", size.HumanReadable()).Info("fetcher: file info")
	}

	f.delivery <- page
}

func (f *FetcherSimple) parseFileName(resp *http.Response, page *Page) error {
	if header := resp.Header.Get("Content-Disposition"); header != "" {
		_, params, err := mime.ParseMediaType(header)

		if err != nil {
			return err
		}

		page.FileName = params["filename"]
	}

	// no header or empty filename
	if page.FileName == "" {
		page.FileName = path.Base(resp.Request.URL.Path)
	}

	return nil
}

func (f *FetcherSimple) dropPage(page *Page, err error) {
	logrus.WithField("full_url", page.FullUrl).WithError(err).Error("fetcher: error fetching")

	page.err = err
	select {
	case f.errors <- page:
		return
	default:
		logrus.Error("fetcher: no err pages listeners. Dropping page with error!")
	}
}

func (f *FetcherSimple) Delivery() <-chan *Page {
	return f.delivery
}

func (f *FetcherSimple) Errors() <-chan *Page {
	return f.errors
}

func (f *FetcherSimple) Queue(page *Page) {
	f.queue <- page
}
