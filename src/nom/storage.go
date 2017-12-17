package main

type Storage interface {
	Get(url string) *Page
	Put(page *Page)
	Iterate() <-chan *Page // iterate over all pages in storage
}
