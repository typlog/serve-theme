package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type resultResponse struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	HTML    string  `json:"html"`
	Time    float64 `json:"ms"`
}

type requestPayload struct {
	Filename string `json:"filename"`
	Code     string `json:"code"`
	URL      string `json:"url"`
}

func serveAny(s http.Handler, root string) http.Handler {
	reArchive := regexp.MustCompile(`^/(archive|posts|episodes|en|zh|ja)/(\d{4}/)?$`)
	reTag := regexp.MustCompile(`^/tags/([^/]+)`)
	reAuthor := regexp.MustCompile(`^/by/([^/])+/(\d{4}/)?$`)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		if r.URL.Path == "/" {
			renderView(root, "home.j2", w, r)
		} else if reArchive.MatchString(r.URL.Path) {
			renderView(root, "list.j2", w, r)
		} else if reTag.MatchString(r.URL.Path) {
			renderView(root, "list.j2", w, r)
		} else if reAuthor.MatchString(r.URL.Path) {
			renderView(root, "list.j2", w, r)
		} else {
			var isAssets bool = false
			suffixes := [...]string{".css", ".js", ".ico", ".jpg", ".png", ".svg", ".woff", ".woff2"}
			for i := 0; i < len(suffixes); i++ {
				if strings.HasSuffix(r.URL.Path, suffixes[i]) {
					isAssets = true
					break
				}
			}
			if isAssets {
				s.ServeHTTP(w, r)
			} else {
				renderView(root, "item.j2", w, r)
			}
		}
	})
}

func renderView(root string, filename string, w http.ResponseWriter, r *http.Request) {
	resp := sendRequest(root, filename, r.URL.Path)
	if resp == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("server error"))
		return
	}
	defer resp.Body.Close()

	var result resultResponse
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err == nil {
		if result.Status == "ok" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(result.HTML))
			log.Println("RENDER: " + r.URL.Path + " - " + strconv.FormatFloat(result.Time, 'f', 5, 64) + "ms")
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(result.Message))
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("unknown error"))
		log.Println(err)
	}
}

func sendRequest(root string, filename string, path string) *http.Response {
	envAPI := os.Getenv("API")
	token := os.Getenv("TOKEN")
	siteId := os.Getenv("SITE")

	var endpoint string
	if envAPI == "" {
		endpoint = "https://api.typlog.com/v3/design/preview"
	} else {
		endpoint = envAPI + "/v3/design/preview"
	}
	byteContent, _ := ioutil.ReadFile(filepath.Join(root, filename))
	strContent := string(byteContent)

	// {{ static_url }} -> /
	reStatic := regexp.MustCompile(`\{\{\s*static_url\s*\}\}`)
	strContent2 := reStatic.ReplaceAllString(strContent, "/")

	// {% include "./_side.j2" %}
	reInclude := regexp.MustCompile(`{% include\s+("|')\./(.+\.j2)("|')\s+%}`)
	var readInclude = func(src string) string {
		names := reInclude.FindStringSubmatch(src)
		content, err := ioutil.ReadFile(filepath.Join(root, names[2]))
		if err == nil {
			return string(content)
		} else {
			return "<pre>**ERROR**: <code>{% raw %}" + src + "{% endraw %}</code></pre>"
		}
	}
	strContent3 := reInclude.ReplaceAllStringFunc(strContent2, readInclude)

	byteBody, _ := json.Marshal(requestPayload{
		Filename: filename,
		URL:      path,
		Code:     strContent3,
	})
	body := string(byteBody)

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(body))
	req.Header.Add("X-Site-Id", siteId)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Length", strconv.Itoa(len(body)))
	req.Header.Set("User-Agent", "ServeTheme/0.2")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	return resp
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	staticHandler := http.FileServer(http.Dir(root))
	http.Handle("/", serveAny(staticHandler, root))

	port := os.Getenv("PORT")
	if port == "" {
		port = "7000"
	}

	log.Println("Listening " + root + " on port " + port)
	log.Fatal(http.ListenAndServe("localhost:"+port, nil))
}
