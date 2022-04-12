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

type configResponse struct {
	ListURL      string `json:"list_permalink"`
	LangURL      string `json:"lang_permalink"`
	TagURL       string `json:"tag_permalink"`
	AuthorURL    string `json:"author_permalink"`
	PostListURL  string `json:"post_list_permalink"`
	AudioListURL string `json:"audio_list_permalink"`
}

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
	resp := request("GET", "design/config", "")

	if resp == nil {
		log.Fatal("Cannot fetch site configuration.")
	}
	defer resp.Body.Close()

	var config configResponse
	err := json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		log.Fatal("Cannot parse site configuration.")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		if r.URL.Path == "/" {
			renderView(root, "home.j2", "", w, r)
		} else if strings.HasPrefix(r.URL.Path, "/-/") {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte(""))
		} else if matchListRoute(config.ListURL, r.URL.Path) {
			renderView(root, "list.j2", "", w, r)
		} else if matchListRoute(config.PostListURL, r.URL.Path) {
			renderView(root, "post_list.j2", "list.j2", w, r)
		} else if matchListRoute(config.AudioListURL, r.URL.Path) {
			renderView(root, "audio_list.j2", "list.j2", w, r)
		} else if matchLangRoute(config.LangURL, r.URL.Path) {
			renderView(root, "lang.j2", "list.j2", w, r)
		} else if matchTagRoute(config.TagURL, r.URL.Path) {
			renderView(root, "tag.j2", "list.j2", w, r)
		} else if matchAuthorRoute(config.AuthorURL, r.URL.Path) {
			renderView(root, "author.j2", "list.j2", w, r)
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
				renderView(root, "item.j2", "", w, r)
			}
		}
	})
}

func matchListRoute(permalink string, path string) bool {
	if path == permalink {
		return true
	}
	reRoute := regexp.MustCompile("^" + permalink + `\d{4}/$`)
	return reRoute.MatchString(path)
}

func matchLangRoute(permalink string, path string) bool {
	langPath := strings.Replace(permalink, "{lang}", "(en|zh|ja|zh-hans|zh-hant|es)", 1)
	reRoute := regexp.MustCompile("^" + langPath + `(\d{4}/)?$`)
	return reRoute.MatchString(path)
}

func matchTagRoute(permalink string, path string) bool {
	langPath := strings.Replace(permalink, "{lang}", "(en|zh|ja|zh-hans|zh-hant|es)", 1)
	tagPath := strings.Replace(langPath, "{slug}", "[a-z0-9-%]+", 1)
	reRoute := regexp.MustCompile("^" + tagPath + `(\d{4}/)?$`)
	return reRoute.MatchString(path)
}

func matchAuthorRoute(permalink string, path string) bool {
	authorPath := strings.Replace(permalink, "{username}", "[a-z0-9-%]+", 1)
	reRoute := regexp.MustCompile("^" + authorPath + `(\d{4}/)?$`)
	return reRoute.MatchString(path)
}

func renderView(root string, filename string, fallback string, w http.ResponseWriter, r *http.Request) {
	var strContent string
	var template string = filename
	byteContent, err1 := ioutil.ReadFile(filepath.Join(root, filename))
	if err1 != nil && fallback != "" {
		template = fallback
		fallbackContent, _ := ioutil.ReadFile(filepath.Join(root, fallback))
		strContent = string(fallbackContent)
	} else {
		strContent = string(byteContent)
	}

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
	strContent2 := reInclude.ReplaceAllStringFunc(strContent, readInclude)

	// {{ static_url }} -> /
	reStatic := regexp.MustCompile(`\{\{\s*static_url\s*\}\}`)
	strContent3 := reStatic.ReplaceAllString(strContent2, "/")


	var _filename string = filename
	if fallback != "" {
		_filename = fallback
	}
	byteBody, _ := json.Marshal(requestPayload{
		Filename: _filename,
		URL:      r.URL.Path,
		Code:     strContent3,
	})
	resp := request("POST", "design/preview", string(byteBody))

	if resp == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("server error"))
		return
	}
	defer resp.Body.Close()

	var result resultResponse
	err2 := json.NewDecoder(resp.Body).Decode(&result)
	if err2 == nil {
		if result.Status == "ok" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(result.HTML))
			log.Println("RENDER: [" + template + "] " + r.URL.Path + " - " + strconv.FormatFloat(result.Time, 'f', 5, 64) + "ms")
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(result.Message))
			log.Println("RENDER: [" + template + "] " + r.URL.Path)
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("unknown error"))
		log.Println(err2)
	}
}

func request(method string, path string, body string) *http.Response {
	envAPI := os.Getenv("API")
	token := os.Getenv("TOKEN")
	siteId := os.Getenv("SITE")

	var endpoint string
	if envAPI == "" {
		endpoint = "https://api.typlog.com/v3/"
	} else {
		endpoint = envAPI + "/v3/"
	}

	req, _ := http.NewRequest(method, endpoint+path, strings.NewReader(body))
	req.Header.Add("X-Site-Id", siteId)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	if body != "" {
		req.Header.Add("Content-Length", strconv.Itoa(len(body)))
	}
	req.Header.Set("User-Agent", "ServeTheme/0.2.1")
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
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
