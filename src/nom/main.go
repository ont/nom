package main

import (
	//"fmt"
	"github.com/davecgh/go-spew/spew"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"log"
	"time"
)

var (
	config = kingpin.Flag("config", "Config file which describes pages and entities for parsing").File()
)

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()

	data, err := ioutil.ReadAll(*config)
	if err != nil {
		log.Fatalln("Error during reading config file: ", err)
	}

	grammar, err := parseConfig(string(data))
	if err != nil {
		log.Fatalln("Error parsing config file: ", err)
	}

	spew.Dump(grammar)

	storage := &StorageFiles{
		Base: "./cache",
	}

	fetcher, err := NewFetcherSimple("http://mnogonot.ucoz.ru", 10)
	if err != nil {
		log.Fatalln("Error creating fetcher: ", err)
	}

	logist := NewLogist(fetcher, storage)
	parser := NewParser(grammar, logist)

	go parser.Start()

	parser.Queue(&Page{
		Name: "content",
		Url:  "/load/partitury/11",
	})

	for {
		time.Sleep(1 * time.Second)
	}
}
