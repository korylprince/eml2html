package eml2html

import (
	"strings"

	"github.com/jhillyerd/enmime"
)

const ContentTypeTextPlain = "text/plain"
const ContentTypeTextHTML = "text/html"
const ContentTypeMessageRFC822 = "message/rfc822"

const ContentTypeMultipartPrefix = "multipart/"
const ContentTypeMultipartAlternative = "multipart/alternative"
const ContentTypeMultipartMixed = "multipart/mixed"
const ContentTypeMultipartRelated = "multipart/related"
const ContentTypeMultipartReport = "multipart/report"

func subParts(m *enmime.Part) []*enmime.Part {
	var parts []*enmime.Part
	for c := m.FirstChild; c != nil; c = c.NextSibling {
		parts = append(parts, c)
	}
	return parts
}

func findMainContent(m *enmime.Part, typ string) [][]byte {
	if m.ContentType == typ {
		return [][]byte{m.Content}
	}
	if !strings.HasPrefix(m.ContentType, ContentTypeMultipartPrefix) {
		return nil
	}

	switch m.ContentType {
	case ContentTypeMultipartAlternative:
		for _, sub := range subParts(m) {
			if sub.ContentType == typ {
				return [][]byte{sub.Content}
			}
		}
		for _, sub := range subParts(m) {
			if sub.ContentType == ContentTypeMultipartRelated {
				return findMainContent(sub, typ)
			}
		}
		for _, sub := range subParts(m) {
			if strings.HasPrefix(sub.ContentType, ContentTypeMultipartPrefix) {
				return findMainContent(sub, typ)
			}
		}

	case ContentTypeMultipartRelated:
		for _, sub := range subParts(m) {
			if sub.ContentType == typ {
				return [][]byte{sub.Content}
			}
		}
		for _, sub := range subParts(m) {
			if sub.ContentType == ContentTypeMultipartAlternative {
				return findMainContent(sub, typ)
			}
		}
		for _, sub := range subParts(m) {
			if strings.HasPrefix(sub.ContentType, ContentTypeMultipartPrefix) {
				return findMainContent(sub, typ)
			}
		}

	case ContentTypeMultipartReport:
		for _, sub := range subParts(m) {
			if sub.ContentType == typ {
				return [][]byte{sub.Content}
			}
		}
		for _, sub := range subParts(m) {
			if sub.ContentType == ContentTypeMultipartRelated {
				return findMainContent(sub, typ)
			}
		}

	case ContentTypeMultipartMixed:
		fallthrough
	default:
		var body [][]byte
		for _, sub := range subParts(m) {
			if sub.ContentType == typ {
				body = append(body, sub.Content)
			} else if strings.HasPrefix(sub.ContentType, ContentTypeMultipartPrefix) {
				body = append(body, findMainContent(sub, typ)...)
			}
		}
		return body
	}

	return nil
}
