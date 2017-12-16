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
	config   = kingpin.Flag("config", "Config file which describes pages and entities for parsing.").File()
	delay    = kingpin.Flag("delay", "Delay between pages fetching.").Default("10").Int()
	cache    = kingpin.Flag("cache", "Cache for fetched and possibly parsed pages.").Default("./cache").String()
	startUrl = kingpin.Arg("url", "Starting url to start parsing from.").Required().String()
	name     = kingpin.Arg("name", "Name of page in config file.").Required().String()
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
		Base: *cache,
	}

	fetcher, err := NewFetcherSimple(*startUrl, *delay)
	if err != nil {
		log.Fatalln("Error creating fetcher: ", err)
	}

	logist := NewLogist(fetcher, storage)
	parser := NewParser(grammar, logist)

	parser.Queue(&Page{
		Name: *name,
		Url:  *startUrl, // TODO: convert to relative?
	})

	// TODO: replace for-loop and go-routine with simple method call (parser.StartAndWait())
	go parser.Start()

	for {
		time.Sleep(1 * time.Second)
	}
}
