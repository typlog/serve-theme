package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

func archiveHandler(w http.ResponseWriter, r *http.Request) {
	renderView("list.j2", w, r)
}

func serveAny(s http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		if r.URL.Path == "/" {
			renderView("home.j2", w, r)
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
				renderView("item.j2", w, r)
			}
		}
	})
}

func renderView(filename string, w http.ResponseWriter, r *http.Request) {
	resp := sendRequest(filename, r.URL.Path)
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

func sendRequest(filename string, path string) *http.Response {
	envAPI := os.Getenv("API")
	token := os.Getenv("TOKEN")
	siteId := os.Getenv("SITE")

	var endpoint string
	if envAPI == "" {
		endpoint = "https://api.typlog.com/v3/design/preview"
	} else {
		endpoint = envAPI + "/v3/design/preview"
	}
	byteContent, _ := ioutil.ReadFile(filename)
	strContent := string(byteContent)

	// {{ static_url }} -> /
	reStatic := regexp.MustCompile(`\{\{\s*static_url\s*\}\}`)
	strContent2 := reStatic.ReplaceAllString(strContent, "/")

	// {% include gh:typlog/ueno/_side.j2 %}
	reInclude := regexp.MustCompile(`{% include\s+("|')gh:[^/]+/[^/]+/(.+\.j2)("|')\s+%}`)
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
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	return resp
}

func readInclude(src string) string {
	re := regexp.MustCompile(`gh:[^/]+/[^/]+/(.+\.j2)`)
	names := re.FindStringSubmatch(src)
	content, err := ioutil.ReadFile(names[1])
	if err == nil {
		return string(content)
	} else {
		return "<pre>**ERROR**: <code>{% raw %}" + src + "{% endraw %}</code></pre>"
	}
}

func main() {
	staticHandler := http.FileServer(http.Dir("."))
	http.HandleFunc("/archive/", archiveHandler)
	http.HandleFunc("/posts/", archiveHandler)
	http.HandleFunc("/episodes/", archiveHandler)
	http.HandleFunc("/by/", archiveHandler)

	langs := [...]string{"en", "zh", "ja"}
	for i := 0; i < len(langs); i++ {
		http.HandleFunc("/"+langs[i]+"/", archiveHandler)
	}

	http.Handle("/", serveAny(staticHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "7000"
	}

	log.Println("Listening on port " + port)
	log.Fatal(http.ListenAndServe("localhost:"+port, nil))
}
