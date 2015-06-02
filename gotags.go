// The gotags command prints a list of all build tags in use and a
// list of all files they apply to.
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var tagFilter = flag.String("tag", "", "only show files with this build tag")

func main() {
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var wg sync.WaitGroup
	for _, root := range paths {
		err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if fi.Mode().IsDir() {
				if name := fi.Name(); path != root && (name[0] == '_' || name[0] == '.') {
					return filepath.SkipDir
				}
			} else if fi.Mode().IsRegular() && filepath.Ext(fi.Name()) == ".go" {
				wg.Add(1)
				go func() {
					defer wg.Done()
					scanFileBuildTags(path)
				}()
			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	wg.Wait()
	printFiles(tagFiles)
}

var (
	tagFiles = map[string][]string{}
	mu       sync.Mutex // guards tagFiles

)

func printFiles(tagFiles map[string][]string) {
	tags := make([]string, 0, len(tagFiles))
	for tag, files := range tagFiles {
		tags = append(tags, tag)
		sort.Strings(files)
	}
	sort.Strings(tags)

	for i, tag := range tags {
		if *tagFilter != "" && !strings.Contains(tag, *tagFilter) {
			continue
		}
		if i != 0 {
			fmt.Println()
		}
		fmt.Println(tag)
		for _, file := range tagFiles[tag] {
			fmt.Print("\t", file, "\n")
		}
	}
}

func scanFileBuildTags(file string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("Parsing %s: %s", file, err)
	}

	for _, cg := range f.Comments {
		txt := cg.Text()
		if strings.HasPrefix(txt, "+build") {
			txt = txt[len("+build "):]
			tags := strings.FieldsFunc(txt, func(r rune) bool {
				return r == ',' || r == ' '
			})
			mu.Lock()
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				tagFiles[tag] = append(tagFiles[tag], file)
			}
			mu.Unlock()
		}
	}
}
