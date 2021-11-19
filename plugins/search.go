package plugins

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"gopkg.in/irc.v3"
)

func init() {
	Register(Search{})
}

type Search struct{}

func (Search) Triggers() []string {
	return []string{".ddg", ".g", ".mdn", ".wiki"}
}

var ddgClient = &http.Client{
	Timeout: 10 * time.Second,
}

func ddgParse(body io.Reader) (string, error) {
	inResult := false
	href := ""
	var snippet strings.Builder
	tok := html.NewTokenizer(body)

	for tt := tok.Next(); tt != html.ErrorToken; tt = tok.Next() {
		switch tt {
		case html.StartTagToken:
			tag, hasAttr := tok.TagName()

			if tag[0] == 'a' {
				if hasAttr {
					var key []byte
					var val []byte
					var lhref []byte
					isRes := false

					for hasAttr {
						key, val, hasAttr = tok.TagAttr()
						if bytes.Equal(key, []byte("href")) {
							lhref = val
						} else if bytes.Equal(val, []byte("result__snippet")) {
							isRes = true
						}
					}

					if isRes && len(lhref) != 0 {
						realUrl, err := url.Parse(string(lhref))
						if err == nil {
							if v, ok := realUrl.Query()["uddg"]; len(v) > 0 && ok {
								inResult = true
								href = v[0]
							}
						}
					}
				}
			} else if tag[0] == 'b' { // support bold text
				//snippet.WriteRune('\x02')
			}
		case html.TextToken:
			if inResult && snippet.Len() < 300 {
				snippet.Write(tok.Text())
			}
		case html.EndTagToken:
			tag, _ := tok.TagName()

			if inResult && tag[0] == 'a' {
				var len int
				if snippet.Len() > 300 {
					len = 300
				} else {
					len = snippet.Len()
				}
				return snippet.String()[:len] + " - " + href, nil
			} else if inResult && tag[0] == 'b' {
				//snippet.WriteRune('\x02')
			}
		}
	}

	if err := tok.Err(); err != io.EOF && err != nil {
		return "HTML parse error", err
	} else {
		return "No results.", nil
	}
}

// Use DuckDuckGo's
func ddg(query string) (string, error) {
	req, err := http.NewRequest("GET", "https://html.duckduckgo.com/html", nil)

	if err != nil {
		return "Client request error", err
	}

	req.Header.Add("User-Agent", "github.com/icyphox/paprika")

	q := req.URL.Query()
	q.Add("q", query)
	req.URL.RawQuery = q.Encode()

	res, err := ddgClient.Do(req)
	if err != nil {
		return "Server response error", err
	}

	defer res.Body.Close()
	result, err := ddgParse(res.Body)
	if err != nil {
		return "HTML parse error", err
	}
	return result, nil
}

// This is just an alias for now.
func google(query string) (string, error) {
	return ddg(query)
}

func mdn(query string) (string, error) {
	return ddg("site:https://developer.mozilla.org/en-US " + query)
}

func wiki(query string) (string, error) {
	return ddg("site:https://en.wikipedia.org " + query)
}

func (Search) Execute(m *irc.Message) (string, error) {
	parsed := strings.SplitN(m.Trailing(), " ", 2)
	if len(parsed) != 2 {
		return fmt.Sprintf("Usage: %s <query>", parsed[0]), nil
	}
	trigger, query := parsed[0], parsed[1]

	switch trigger {
	case ".mdn":
		return mdn(query)
	case ".wiki":
		return wiki(query)
	case ".g":
		return google(query)
	default:
		return ddg(query)
	}
}
