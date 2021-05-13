package html

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/mail"
	"os"
	"path/filepath"
	"sort"

	"github.com/jhillyerd/enmime"
)

var listTmpl = template.Must(template.ParseFS(tmpls, "templates/meta.tmpl", "templates/list.tmpl", "templates/base.tmpl"))

type dirEntry struct {
	Name  string
	Msg   *enmime.Envelope
	Error error
	info  fs.FileInfo
}

//WriteDir parses and writes all of the given files in a directory with the given name at root.
//meta is used to provide navigational links to neighbors or a parent.
//WriteDir writes links to sub directories, but does not parse them. Subsequent calls to WriteDir should be called for each sub directory
func WriteDir(root, name string, files []fs.File, meta *Meta) error {
	root = filepath.Join(root, name)

	if err := os.MkdirAll(root, modeDir); err != nil {
		return fmt.Errorf("Unable to create root directory: %w", err)
	}

	//build files
	entries := make([]*dirEntry, 0, len(files))
	for idx, f := range files {
		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("Unable to stat file %d: %w", idx, err)
		}

		name := info.Name()

		if info.IsDir() {
			entries = append(entries, &dirEntry{Name: name, info: info})
			continue
		}

		msg, err := writeMsgRoot(root, name, f)
		if err != nil {
			entries = append(entries, &dirEntry{Name: name, Error: fmt.Errorf("Unable to write msg %s root: %w", name, err), info: info})
			continue
		}

		entries = append(entries, &dirEntry{Name: name, Msg: msg, info: info})
	}

	//sort files
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Msg == nil {
			if entries[j].Msg == nil {
				return entries[i].Name < entries[j].Name
			}
			return true
		}
		if entries[j].Msg == nil {
			return false
		}
		it, err := mail.ParseDate(entries[i].Msg.GetHeader("Date"))
		if err != nil {
			return true
		}
		jt, err := mail.ParseDate(entries[j].Msg.GetHeader("Date"))
		if err != nil {
			return false
		}
		return it.Before(jt)
	})

	//build meta
	metas := make(map[int]string)
	for idx, e := range entries {
		metas[idx] = fmt.Sprintf("../%s/index.html", e.info.Name())
	}

	for idx, e := range entries {
		meta := &Meta{Parent: "../index.html"}
		if url, ok := metas[idx-1]; ok {
			meta.Prev = url
		}
		if url, ok := metas[idx+1]; ok {
			meta.Next = url
		}

		if e.info.IsDir() {
			continue
		}

		if e.Error != nil {
			if err := os.WriteFile(filepath.Join(root, e.info.Name(), "error.txt"), []byte(e.Error.Error()), modeFile); err != nil {
				e.Error = fmt.Errorf("Unable to create write error.txt: %w; original error %v", err, e.Error)
			}
		}

		if err := WriteMsg(root, e.Name, e.Msg, meta); err != nil {
			e.Error = err
			continue
		}
	}

	if err := writeDirIndex(root, name, entries, meta); err != nil {
		return fmt.Errorf("Unable to write: %w", err)
	}

	return nil
}

func writeDirIndex(root, name string, entries []*dirEntry, meta *Meta) error {
	buf := new(bytes.Buffer)

	if err := listTmpl.ExecuteTemplate(buf, "base.tmpl",
		struct {
			Meta    *Meta
			Name    string
			Entries []*dirEntry
		}{meta, name, entries},
	); err != nil {
		panic(fmt.Errorf("Unable to execute template: %w", err))
	}

	if err := os.WriteFile(filepath.Join(root, "index.html"), buf.Bytes(), modeFile); err != nil {
		return fmt.Errorf("Unable to create write index.html: %w", err)
	}

	return nil
}
