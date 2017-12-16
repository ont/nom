package main

type Storage interface {
	Get(url string) *Page
	Put(page *Page)
}
