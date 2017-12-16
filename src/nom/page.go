package main

import (
	"crypto/md5"
	"fmt"
	"reflect"

	"github.com/vmihailenco/msgpack"
)

type Page struct {
	Name string // name of config block to parse with

	Url         string // url as it was in parsed html
	ReferrerUrl string // full url of referrer page (for relative page.Url resolving)
	FullUrl     string // start url from which page was really downloaded (page.Url normalized inside fetcher)
	FinalUrl    string // final url after all redirects

	IsFile   bool   // true - if Body contains bytes of downloaded file
	FileName string // filename (from Content-Disposition header or FinalUrl)

	Body []byte // raw content of downloaded page  (TODO: gzip it)

	err error // last error associated with page

	Tree *Block // parsed blocks on page

	Hash []byte // md5 sum of all struct fields (for changes detection)
}

type Block struct {
	Fields map[string][]ValueOrBlock
}

type ValueOrBlock interface{}

func NewPage(bytes []byte) *Page {
	var page Page
	err := msgpack.Unmarshal(bytes, &page)
	if err != nil {
		return nil
	}

	return &page
}

func (p *Page) Serialize() ([]byte, error) {
	return msgpack.Marshal(p)
}

func (p *Page) ContentHash() []byte {
	hasher := md5.New()

	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).CanSet() {
			continue // skip unexported fields
		}

		if v.Type().Field(i).Name == "Hash" {
			continue // skip hash itself
		}

		value := v.Field(i).Interface()
		bytes := []byte(fmt.Sprintf("%v", value)) // TODO: very slow, manual hasher.Write(struct.<some-field>) ??
		hasher.Write(bytes)
	}

	return hasher.Sum(nil)
}

func (p *Page) IsChanged() bool {
	currentHash := p.ContentHash()

	if len(currentHash) != len(p.Hash) {
		return true
	}

	for i, b := range p.Hash {
		if currentHash[i] != b {
			return true
		}
	}

	return false
}

func (p *Page) UpdateHash() {
	p.Hash = p.ContentHash()
}
