package main

import (
	"github.com/sirupsen/logrus"
)

type Logist struct {
	fetcher Fetcher
	storage Storage

	delivery chan *Page
}

func NewLogist(fetcher Fetcher, storage Storage) *Logist {
	return &Logist{
		fetcher: fetcher,
		storage: storage,

		delivery: make(chan *Page),
	}
}

func (l *Logist) Fetch(page *Page) {
	log := logrus.WithField("url", page.Url)
	savedPage := l.storage.Get(page.Url)
	if savedPage != nil {
		log.Info("logist: from storage")
		l.delivery <- savedPage
		return
	}

	log.Info("logist: to fetcher")
	l.fetcher.Queue(page)
}

func (l *Logist) Store(page *Page) {
	if page.IsChanged() {
		logrus.WithField("url", page.Url).Info("logist: page changed, storing...")

		page.UpdateHash()
		l.storage.Put(page)
	} else {
		logrus.WithField("url", page.Url).Info("logist: skip saving (no changes)")
	}
}

func (l *Logist) Delivery() <-chan *Page {
	return l.delivery
}

func (l *Logist) Errors() <-chan *Page {
	return l.fetcher.Errors()
}

func (l *Logist) Start() {
	logrus.Info("logist: starting")

	go l.processFetcher()
	go l.fetcher.Start()
}

func (l *Logist) processFetcher() {
	for page := range l.fetcher.Delivery() {
		logrus.WithField("url", page.Url).Info("logist: saving to storage")
		l.storage.Put(page)
		l.delivery <- page
	}
}
