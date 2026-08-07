package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const wikiAPI = "https://stardewvalleywiki.com/mediawiki/api.php"

type parseResp struct {
	Parse struct {
		Wikitext struct {
			Star string `json:"*"`
		} `json:"wikitext"`
	} `json:"parse"`
}

func fetchWikitextFrom(api, page string) (string, error) {
	u := fmt.Sprintf("%s?action=parse&prop=wikitext&format=json&page=%s", api, url.QueryEscape(page))
	resp, err := http.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var pr parseResp
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", fmt.Errorf("parsing API response: %w", err)
	}
	if pr.Parse.Wikitext.Star == "" {
		return "", fmt.Errorf("empty wikitext for page %s", page)
	}
	return pr.Parse.Wikitext.Star, nil
}

func fetchWikitext(page string) (string, error) { return fetchWikitextFrom(wikiAPI, page) }

// titlesPerRequest is MediaWiki's cap on the `titles` parameter.
const titlesPerRequest = 50

type queryResp struct {
	Query struct {
		Normalized []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"normalized"`
		Redirects []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"redirects"`
		Pages map[string]struct {
			Title     string `json:"title"`
			Revisions []struct {
				Slots struct {
					Main struct {
						Star string `json:"*"`
					} `json:"main"`
				} `json:"slots"`
			} `json:"revisions"`
		} `json:"pages"`
	} `json:"query"`
}

type listResp struct {
	Continue map[string]string `json:"continue"`
	Query    struct {
		Allpages []struct {
			Title string `json:"title"`
		} `json:"allpages"`
		Embeddedin []struct {
			Title string `json:"title"`
		} `json:"embeddedin"`
	} `json:"query"`
}

func fetchList(api string, params url.Values, key string) ([]string, error) {
	var titles []string
	for {
		params.Set("action", "query")
		params.Set("format", "json")
		u := api + "?" + params.Encode()
		resp, err := http.Get(u)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var result listResp
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing API response: %w", err)
		}
		if key == "allpages" {
			for _, entry := range result.Query.Allpages {
				titles = append(titles, entry.Title)
			}
		} else {
			for _, entry := range result.Query.Embeddedin {
				titles = append(titles, entry.Title)
			}
		}
		if len(result.Continue) == 0 {
			return titles, nil
		}
		for continuationKey, value := range result.Continue {
			params.Set(continuationKey, value)
		}
	}
}

func fetchInfoboxTemplatesFrom(api string) ([]string, error) {
	titles, err := fetchList(api, url.Values{"list": {"allpages"}, "apnamespace": {"10"}, "apprefix": {"Infobox"}, "aplimit": {"max"}}, "allpages")
	if err != nil {
		return nil, err
	}
	var templates []string
	for _, title := range titles {
		if strings.HasPrefix(strings.ToLower(title), "template:infobox") {
			templates = append(templates, title)
		}
	}
	return templates, nil
}

func fetchEmbeddedPagesFrom(api, template string) ([]string, error) {
	return fetchList(api, url.Values{"list": {"embeddedin"}, "eititle": {template}, "einamespace": {"0"}, "eilimit": {"max"}}, "embeddedin")
}

func fetchItemPages() (map[string]string, error) {
	templates, err := fetchInfoboxTemplatesFrom(wikiAPI)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var titles []string
	for _, template := range templates {
		pages, err := fetchEmbeddedPagesFrom(wikiAPI, template)
		if err != nil {
			return nil, err
		}
		for _, title := range pages {
			if !seen[title] {
				seen[title] = true
				titles = append(titles, title)
			}
		}
	}
	return fetchPages(titles)
}

// fetchPagesFrom returns the wikitext of each requested page, keyed by the
// title as requested. The wiki may normalize ("omelet" → "Omelet") or redirect
// ("Cookies" → "Cookie") a title on the way, so those hops are followed back
// to the caller's spelling. Titles with no page are simply absent.
func fetchPagesFrom(api string, titles []string) (map[string]string, error) {
	out := make(map[string]string, len(titles))
	for start := 0; start < len(titles); start += titlesPerRequest {
		end := min(start+titlesPerRequest, len(titles))
		batch := titles[start:end]

		u := fmt.Sprintf("%s?action=query&prop=revisions&rvprop=content&rvslots=main&format=json&redirects=1&titles=%s",
			api, url.QueryEscape(strings.Join(batch, "|")))
		resp, err := http.Get(u)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var qr queryResp
		if err := json.Unmarshal(body, &qr); err != nil {
			return nil, fmt.Errorf("parsing API response: %w", err)
		}

		// Walk each requested title forward through normalization and
		// redirects to the title the response actually keys content under.
		hop := map[string]string{}
		for _, n := range qr.Query.Normalized {
			hop[n.From] = n.To
		}
		for _, r := range qr.Query.Redirects {
			hop[r.From] = r.To
		}
		byTitle := map[string]string{}
		for _, p := range qr.Query.Pages {
			if len(p.Revisions) > 0 {
				byTitle[p.Title] = p.Revisions[0].Slots.Main.Star
			}
		}
		for _, want := range batch {
			final := want
			for i := 0; i < 5; i++ {
				next, ok := hop[final]
				if !ok {
					break
				}
				final = next
			}
			if text, ok := byTitle[final]; ok {
				out[want] = text
			}
		}
	}
	return out, nil
}

func fetchPages(titles []string) (map[string]string, error) {
	return fetchPagesFrom(wikiAPI, titles)
}
