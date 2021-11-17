package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	html "github.com/korylprince/eml2html"
	walk "github.com/korylprince/go-fs-walk"
)

func emlFilter(fi fs.FileInfo, path string, err error) error {
	if fi.IsDir() {
		return nil
	}
	if !strings.HasSuffix(fi.Name(), ".eml") {
		return walk.Skip
	}
	return nil
}

type file struct {
	r  io.Reader
	fi fs.FileInfo
}

func (f *file) Stat() (fs.FileInfo, error) {
	return f.fi, nil
}

func (f *file) Read(b []byte) (int, error) {
	return f.r.Read(b)
}

func (f *file) Close() error {
	return nil
}

func usage() {
	fmt.Printf("Usage: %s -in /path/to/in -out /path/to/out\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	input := flag.String("in", "", "input directory or file")
	output := flag.String("out", "", "output directory for html. Will be created if it doesn't exist")
	flag.Usage = usage
	flag.Parse()

	if *input == "" || *output == "" {
		usage()
		os.Exit(2)
	}

	root := path.Clean(*input)
	chroot := root

	c := walk.New(root)
	c.RegisterFilterFunc("eml", emlFilter)

	dirs := make(map[string][]fs.File)

	for f, p, err := c.Next(); err != io.EOF; f, p, err = c.Next() {
		if err != nil {
			log.Println("WARN: Error walking directory:", err)
			continue
		}

		// if -in is a file, chroot around it's parent
		if p == root && !f.IsDir() {
			chroot = path.Dir(p)
		}

		if f.IsDir() {
			// don't include root's parent if root is directory
			if p == root {
				continue
			}
			dirs[path.Dir(p)] = append(dirs[path.Dir(p)], &file{fi: f})
			continue
		}

		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, c); err != nil {
			log.Println("WARN: Error reading file:", err)
			continue
		}

		dirs[path.Dir(p)] = append(dirs[path.Dir(p)], &file{r: buf, fi: f})
	}

	if err := os.MkdirAll(*output, 0755); err != nil {
		log.Fatalln("Unable to create output directory:", err)
	}

	for d, entries := range dirs {
		var meta *html.Meta
		if d != root {
			meta = &html.Meta{Parent: "../index.html"}
		}
		if err := html.WriteDir(*output, path.Clean(strings.TrimPrefix(d, chroot)), entries, meta); err != nil {
			log.Fatalln("Unable to write directory:", err)
		}
	}
}
