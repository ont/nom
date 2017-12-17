package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
)

type Exporter struct {
	storage   Storage
	tokenizer *regexp.Regexp
	steps     []*ExportStep
}

type ExportStep struct {
	Name   string // page name
	Field  string // field on this page
	Filler string // .. or simply part of path
}

func NewExporter(storage Storage, rule string) *Exporter {
	exporter := &Exporter{
		storage:   storage,
		tokenizer: regexp.MustCompile(`{([^{}]+)}`),
		steps:     make([]*ExportStep, 0),
	}
	exporter.parseRule(rule)

	return exporter
}

func (e *Exporter) parseRule(rule string) {
	for rule != "" {
		loc := e.tokenizer.FindStringIndex(rule)

		filler, token := "", ""

		if loc != nil {
			filler = rule[0:loc[0]]
			token = rule[loc[0]:loc[1]]
			rule = rule[loc[1]:]
			spew.Dump(rule)
		} else {
			filler = rule
			rule = ""
		}

		if filler != "" {
			e.steps = append(e.steps, &ExportStep{
				Filler: filler,
			})
		}

		if token != "" {
			token = strings.Trim(token, "{}")
			arr := strings.Split(token, ":")

			if len(arr) < 2 {
				logrus.Fatalf("Error parsing rule: token must be in format \"page_name:field\", but \"%s\" found", token)
			}

			e.steps = append(e.steps, &ExportStep{
				Name:   arr[0],
				Field:  arr[1],
				Filler: "",
			})
		}
	}

	spew.Dump(e.steps)
}

func (e *Exporter) Export() {
	for page := range e.storage.Iterate() {
		e.exportRecursive("", page, false, e.steps)
	}
}

func (e *Exporter) exportRecursive(path string, page *Page, extractChilds bool, steps []*ExportStep) {
	if len(steps) == 0 {
		err := e.dumpPage(path, page)
		if err != nil {
			logrus.WithField("url", page.Url).WithError(err).Error("exporter: error dumping to file")
			return
		}
		return
	}

	step := steps[0]

	if step.Filler != "" {
		e.exportRecursive(path+step.Filler, page, extractChilds, steps[1:])
		return
	}

	// At token-step we may want to extract all child pages for that token...
	if extractChilds {
		urls, err := e.extractField(page, step.Name)
		if err != nil {
			logrus.WithError(err).Errorf("exporter: error extracting urls for child pages \"%s\" from \"%s\"", step.Name, page.Name)
			return
		}

		for _, url := range urls {
			childPage := e.storage.Get(url)
			if childPage == nil {
				logrus.WithField("url", url).Error("exporter: can't load page")
			}

			// NOTE: we just create multiple branches at token-step,
			// because of that we use "steps" and "extractChilds = false"
			// instead of "steps[1:]" and "extractChilds = true"
			e.exportRecursive(path, childPage, false, steps)
		}
	}

	// Now we actually parse single page (extracted child),
	// here we check that we got correct page after branching and loading from storage.
	if page.Name != step.Name {
		return
	}

	value, err := e.preparePathValue(page, step.Field)
	if err != nil {
		logrus.WithError(err).Errorf("exporter: error extracting field \"%s\"", step.Field)
		return
	}

	e.exportRecursive(path+value, page, true, steps[1:])
}

func (e *Exporter) preparePathValue(page *Page, field string) (string, error) {
	// special field names
	switch field {
	case "ext":
		if !page.IsFile {
			return "", fmt.Errorf("can't find extension for \"%s\", page is not downloadable file", page.Name)
		}

		// TODO: sanitize value (allow only [a-zA-Z'". ])
		return strings.TrimLeft(filepath.Ext(page.FileName), "."), nil

	case "file":
		if !page.IsFile {
			return "", fmt.Errorf("can't find filename for \"%s\", page is not downloadable file", page.Name)
		}

		// TODO: sanitize value (allow only [a-zA-Z'". ])
		return page.FileName, nil

	}

	values, err := e.extractField(page, field)
	if err != nil {
		return "", err
	}

	// TODO: better sanitize value (allow only [a-zA-Z'". ])
	return strings.Replace(strings.Join(values, " "), "/", "-", -1), nil
}

func (e *Exporter) extractField(page *Page, field string) ([]string, error) {
	blocks, found := page.Tree.Fields[field]
	res := make([]string, 0, len(blocks))

	if !found {
		return nil, fmt.Errorf("missed \"%s\" field for page \"%s\"", field, page.Name)
	}

	for _, block := range blocks {
		if value, ok := block.(string); ok {
			res = append(res, value)
		} else {
			return nil, fmt.Errorf("complex block \"%s\" is not supported yet (TODO)", field)
		}
	}

	return res, nil
}

func (e *Exporter) dumpPage(path string, page *Page) error {
	if !page.IsFile {
		return fmt.Errorf("page is not downloaded file and can't be exported (TODO)")
	}

	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)

	return ioutil.WriteFile(path, page.Body, 0644)
}
