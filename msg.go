//package eml2html is a library used to transform RFC 822 (eml) files into html files viewable in a browser.
package eml2html

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhillyerd/enmime"
	"github.com/k3a/html2text"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const modeFile = 0644
const modeDir = 0755 | fs.ModeDir

//go:embed templates/*.tmpl
var tmpls embed.FS

var emailTmpl = template.Must(template.ParseFS(tmpls, "templates/meta.tmpl", "templates/email.tmpl", "templates/base.tmpl"))

//Meta is used to link pages to other pages
type Meta struct {
	Prev   string
	Next   string
	Parent string
}

func writeMsgRoot(root, name string, r io.Reader) (*enmime.Envelope, error) {
	root = filepath.Join(root, name)

	if err := os.MkdirAll(root, modeDir); err != nil {
		return nil, fmt.Errorf("Unable to create root directory: %w", err)
	}

	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("Unable to read file: %w", err)
	}

	if err = os.WriteFile(filepath.Join(root, name), buf, modeFile); err != nil {
		return nil, fmt.Errorf("Unable to create write original: %w", err)
	}

	msg, err := enmime.ReadEnvelope(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("Unable to parse msg: %w", err)
	}

	return msg, nil
}

//WriteMsg parses and writes the given name/reader as an RFC 822 (eml) message and outputs an HTML representation at root.
//meta is used to provide navigational links to neighbors or a parent
func WriteMsg(root, name string, msg *enmime.Envelope, meta *Meta) error {
	root = filepath.Join(root, name)

	if err := writeIndex(root, name, msg, meta); err != nil {
		return fmt.Errorf("Unable to write: %w", err)
	}

	return nil
}

type attachment struct {
	Name string
	Link string
}

func writeAttachments(root string, msg *enmime.Envelope) (attachments []*attachment, contentIDMap map[string]string, err error) {
	root = filepath.Join(root, "attachments")

	if err := os.MkdirAll(root, modeDir); err != nil {
		return nil, nil, fmt.Errorf("Unable to create root attachments directory: %w", err)
	}

	names := make(map[string]int)
	contentIDMap = make(map[string]string)

	for _, msg := range append(msg.Attachments, msg.Inlines...) {
		//embedded message/rfc822
		if msg.ContentType == ContentTypeMessageRFC822 || msg.FileName == "mime-attachment" || strings.HasPrefix(msg.FileName, ".eml") {
			fn := "attached.eml"
			if i, ok := names[fn]; ok {
				ext := filepath.Ext(fn)
				name := strings.TrimSuffix(fn, ext)
				fn = fmt.Sprintf("%s (%d)%s", name, i, ext)
			}
			names[fn]++

			m, err := writeMsgRoot(root, fn, bytes.NewReader(msg.Content))
			if err != nil {
				return nil, nil, fmt.Errorf("Unable to write embedded msg %s root: %w", fn, err)
			}
			if err := WriteMsg(root, fn, m, &Meta{Parent: "../../index.html"}); err != nil {
				return nil, nil, fmt.Errorf("Unable to write embedded msg %s: %w", fn, err)
			}

			attachments = append(attachments, &attachment{Name: fn, Link: fn + "/index.html"})
			continue
		}

		fn := msg.FileName
		if fn == "" {
			continue
		}

		if i, ok := names[fn]; ok {
			ext := filepath.Ext(fn)
			name := strings.TrimSuffix(fn, ext)
			fn = fmt.Sprintf("%s (%d)%s", name, i, ext)
		}
		names[fn]++

		if err := os.WriteFile(filepath.Join(root, fn), msg.Content, modeFile); err != nil {
			return nil, nil, fmt.Errorf("Unable to create write attachment %s: %w", fn, err)
		}

		if msg.ContentID != "" {
			contentIDMap[strings.Trim(msg.ContentID, "<>")] = fn
		}

		attachments = append(attachments, &attachment{Name: fn, Link: fn})
	}

	return attachments, contentIDMap, nil
}

func rewriteHTML(contentIDMap map[string]string, b []byte) ([]byte, error) {
	r := bytes.NewReader(b)
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse html: %w", err)
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		// rewrite charset to utf-8
		if n.DataAtom == atom.Meta {
			for _, attr := range n.Attr {
				if strings.ToLower(attr.Key) == "http-equiv" && strings.ToLower(attr.Val) == "content-type" {
					n.Attr = []html.Attribute{
						{Key: "http-equiv", Val: "Content-Type"},
						{Key: "content", Val: "text/html; charset=utf-8"},
					}
				}
				if strings.ToLower(attr.Key) == "charset" {
					n.Attr = []html.Attribute{
						{Key: "charset", Val: "utf-8"},
					}
				}
			}
		}

		// rewrite links using content map
		for idx, a := range n.Attr {
			if strings.ToLower(a.Key) == "src" && strings.HasPrefix(a.Val, "cid:") {
				id := strings.TrimPrefix(a.Val, "cid:")
				if dst, ok := contentIDMap[id]; ok {
					a.Val = "../attachments/" + dst
					n.Attr[idx] = a
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	buf := new(bytes.Buffer)
	if err = html.Render(buf, doc); err != nil {
		return nil, fmt.Errorf("Unable to render html: %w", err)
	}
	return buf.Bytes(), nil
}

func writeContent(root, name string, msg *enmime.Envelope, contentIDMap map[string]string) (txtContent, htmlContent []string, err error) {
	root = filepath.Join(root, "content")

	if err := os.MkdirAll(root, modeDir); err != nil {
		return nil, nil, fmt.Errorf("Unable to create root content directory: %w", err)
	}

	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	tc := findMainContent(msg.Root, ContentTypeTextPlain)
	hc := findMainContent(msg.Root, ContentTypeTextHTML)

	var cleanTC [][]byte
	for _, t := range tc {
		if strings.TrimSpace(string(t)) != "" {
			cleanTC = append(cleanTC, t)
		}
	}

	var cleanHC [][]byte
	for _, h := range hc {
		txt := html2text.HTML2Text(string(h))
		if strings.TrimSpace(txt) != "" {
			cleanHC = append(cleanHC, h)
		}
	}

	for i, b := range cleanTC {
		fn := fmt.Sprintf("%s.txt", name)
		if len(tc) > 1 {
			fn = fmt.Sprintf("%s_%d.txt", name, i+1)
		}

		if err := os.WriteFile(filepath.Join(root, fn), b, modeFile); err != nil {
			return nil, nil, fmt.Errorf("Unable to create write text content %s: %w", fn, err)
		}

		txtContent = append(txtContent, fn)
	}

	for i, b := range cleanHC {
		h, err := rewriteHTML(contentIDMap, b)
		if err != nil {
			h = b
		}

		fn := fmt.Sprintf("%s.html", name)
		if len(hc) > 1 {
			fn = fmt.Sprintf("%s_%d.html", name, i+1)
		}

		if err := os.WriteFile(filepath.Join(root, fn), h, modeFile); err != nil {
			return nil, nil, fmt.Errorf("Unable to create write html content %s: %w", fn, err)
		}

		htmlContent = append(htmlContent, fn)
	}

	return txtContent, htmlContent, nil
}

func writeIndex(root, name string, msg *enmime.Envelope, meta *Meta) error {
	attachments, contentIDMap, err := writeAttachments(root, msg)
	if err != nil {
		return fmt.Errorf("Unable to write attachments: %w", err)
	}

	txt, html, err := writeContent(root, name, msg, contentIDMap)
	if err != nil {
		return fmt.Errorf("Unable to write content: %w", err)
	}

	buf := new(bytes.Buffer)

	if err := emailTmpl.ExecuteTemplate(buf, "base.tmpl",
		struct {
			Meta        *Meta
			Name        string
			Msg         *enmime.Envelope
			TextContent []string
			HTMLContent []string
			Attachments []*attachment
		}{meta, name, msg, txt, html, attachments},
	); err != nil {
		panic(fmt.Errorf("Unable to execute template: %w", err))
	}

	if err = os.WriteFile(filepath.Join(root, "index.html"), buf.Bytes(), modeFile); err != nil {
		return fmt.Errorf("Unable to create write index.html: %w", err)
	}

	return nil
}
