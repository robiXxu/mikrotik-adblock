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

// delay in ms (delay between chunks)
var delay uint = 3000
var chunkSize uint = 3000
var outputFile string = "./output.txt"
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
	separator := " "
	var entries []Entry
	for _, l := range lines {
		domain := strings.TrimSpace(l)
		if strings.HasPrefix(domain, "#") {
			continue
		}
		if strings.Contains(domain, separator) {
			parts := strings.Split(domain, separator)
			domain = parts[1]
		}
		if len(domain) > 4 {
			entries = append(entries, Entry{
				domain: domain,
			})
		}
	}
	return entries
}

func unique(sample []Entry) []Entry {
	var unique []Entry
	type key struct{ domain string }
	m := make(map[key]int)
	for _, v := range sample {
		k := key{v.domain}
		if i, ok := m[k]; ok {
			unique[i] = v
		} else {
			m[k] = len(unique)
			unique = append(unique, v)
		}
	}
	return unique
}

func chunks(slice []Entry, size uint) [][]Entry {
	var chunks [][]Entry
	for i := 0; i < len(slice); i += int(size) {
		end := i + int(size)
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func writeCommands(chunkEntries [][]Entry, filename string) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatalf("Failed to create file: %+v", err)
	}
	defer file.Close()

	totalAmountOfChunks := len(chunkEntries)
	w := bufio.NewWriter(file)
	address := "127.0.0.1"

	fmt.Fprintln(w, "/log info \"AD BLOCKER\"")
	for no, chunk := range chunkEntries {
		fmt.Fprintln(w, fmt.Sprintf("/log info \"[START] Chunk %d of %d\"", no, totalAmountOfChunks))
		fmt.Fprintln(w, "/ip dns static")
		for _, entry := range chunk {
			fmt.Fprintln(w, fmt.Sprintf("add address=%s name=%s", address, entry.domain))
		}
		fmt.Fprintln(w, fmt.Sprintf("/delay %dms", delay))
		fmt.Fprintln(w, fmt.Sprintf("/log info \"[END] Chunk %d of %d\"", no, totalAmountOfChunks))
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

	filtered := unique(entries)
	log.Printf("Filtered %d", len(filtered))

	wgRead.Wait()

	entriesInChunks := chunks(filtered, chunkSize)

	err = writeCommands(entriesInChunks, outputFile)
	if err != nil {
		log.Fatalf("Failed to write file %s. Error: %+v", outputFile, err)
	}
}
