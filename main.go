package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
)

var wgRead sync.WaitGroup

type Entry struct {
	domain string
}

func readFile(filePath string, out chan<- string, wg *sync.WaitGroup) {
	log.Printf("Reading file (%s)", filePath)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file %s. Error %+v", filePath, err)
	}
	defer wg.Done()
	out <- string(data)
}

func parseData(data string) []Entry {
	log.Printf("Parsing data...")
	lines := strings.Split(data, "\n")
	log.Printf("Got %d rows", len(lines))
	var entries []Entry
	for _, l := range lines {
		domain := strings.TrimSpace(l)
		if len(domain) > 3 {
			entries = append(entries, Entry{
				domain: domain,
			})
		}
	}
	return entries
}

func writeCommand(entries []Entry, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to create file: %+v", err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	fmt.Fprintln(w, "/ip dns static")
	address := "127.0.0.1"
	for _, entry := range entries {
		fmt.Fprintln(w, fmt.Sprintf("add address=%s name=%s", address, entry.domain))
	}
	return w.Flush()
}

func main() {

	dirPath := "./lists/"
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatalf("Failed to read directory %s. Error %+v", dirPath, err)
	}
	var chans = []chan string{}
	for _, f := range files {
		if !f.IsDir() && strings.Contains(f.Name(), "txt") {
			wgRead.Add(1)
			ch := make(chan string, 1)
			chans = append(chans, ch)
			filePath := fmt.Sprintf("%s/%s", dirPath, f.Name())
			go readFile(filePath, ch, &wgRead)
		}
	}

	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}

	var entries []Entry
	remaining := len(cases)
	for remaining > 0 {
		i, value, ok := reflect.Select(cases)
		log.Printf("%+v", ok)
		if !ok {
			cases[i].Chan = reflect.ValueOf(nil)
			remaining -= 1
			continue
		}
		fEntries := parseData(value.String())
		entries = append(entries, fEntries...)
		log.Printf("Read from channel %+v. Got %d valid entries", chans[i], len(entries))

		remaining -= 1
	}

	log.Printf("Total %d", len(entries))
	wgRead.Wait()

	outputFile := "./output.txt"

	err = writeCommand(entries, outputFile)
	if err != nil {
		log.Fatalf("Failed to write file %s. Error: %+v", outputFile, err)
	}
}
