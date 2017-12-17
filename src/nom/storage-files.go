package main

import (
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

type StorageFiles struct {
	Base string
}

func (c *StorageFiles) Get(url string) *Page {
	return c.PageFromFile(c.getPath(url))
}

func (c *StorageFiles) PageFromFile(path string) *Page {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil
	}

	return NewPage(bytes)
}

func (c *StorageFiles) Put(page *Page) {
	if page.Url == "" {
		return
	}

	fname := c.getPath(page.Url)
	err := os.MkdirAll(path.Dir(fname), 0755)
	if err != nil {
		return
	}

	bytes, err := page.Serialize()
	if err != nil {
		return
	}
	ioutil.WriteFile(fname, bytes, 0644)
}

func (c *StorageFiles) getPath(url string) string {
	hash := md5.Sum([]byte(url))
	name := hex.EncodeToString(hash[:])
	return c.Base + "/" + name[0:2] + "/" + name[2:4] + "/" + name + ".pac"
}

func (c *StorageFiles) Iterate() <-chan *Page {
	ch := make(chan *Page)
	go func() {
		filepath.Walk(c.Base, func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() {
				ch <- c.PageFromFile(path)
			}
			return err
		})
		close(ch)
	}()
	return ch
}
