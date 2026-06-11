package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

var tokenRegex = regexp.MustCompile(`\?t=([A-Za-z0-9._-]+)`)

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	rawPath := strings.Trim(r.URL.Path, "/")

	if rawPath == "" || rawPath == "api" {
		http.Error(w, "Error: Provide the file ID or URL in the path", http.StatusBadRequest)
		return
	}

	parts := strings.Split(rawPath, "/")
	fileID := parts[len(parts)-1]

	if fileID == "" {
		http.Error(w, "Error: Could not extract file ID", http.StatusBadRequest)
		return
	}

	domain := "buzzheavier.com"
	if strings.Contains(rawPath, "bzzhr.to") {
		domain = "bzzhr.to"
	}

	baseURL := fmt.Sprintf("https://%s/%s", domain, fileID)

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(15),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithNotFollowRedirects(),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		http.Error(w, "Error: Failed to create TLS client", http.StatusInternalServerError)
		return
	}

	req1, _ := fhttp.NewRequest(fhttp.MethodGet, baseURL, nil)
	req1.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp1, err := client.Do(req1)
	if err != nil {
		http.Error(w, "Error: Failed to reach host", http.StatusBadGateway)
		return
	}
	defer resp1.Body.Close()

	bodyBytes, _ := io.ReadAll(resp1.Body)
	matches := tokenRegex.FindStringSubmatch(string(bodyBytes))
	if len(matches) < 2 {
		http.Error(w, "Error: Security token not found", http.StatusNotFound)
		return
	}
	token := matches[1]

	downloadURL := fmt.Sprintf("%s/download?t=%s", baseURL, token)

	req2, _ := fhttp.NewRequest(fhttp.MethodGet, downloadURL, nil)
	req2.Header.Set("hx-request", "true")
	req2.Header.Set("referer", baseURL)
	req2.Header.Set("accept", "*/*")
	req2.Header.Set("User-Agent", req1.Header.Get("User-Agent"))

	resp2, err := client.Do(req2)
	if err != nil {
		http.Error(w, "Error: Failed to request direct link", http.StatusBadGateway)
		return
	}
	defer resp2.Body.Close()

	finalURL := resp2.Header.Get("Location")
	if finalURL == "" {
		finalURL = resp2.Header.Get("HX-Redirect")
	}

	if finalURL == "" {
		body2Bytes, _ := io.ReadAll(resp2.Body)
		finalURL = string(body2Bytes)
	}

	w.Header().Set("Cache-Control", "s-maxage=345600, stale-while-revalidate")

	fmt.Fprint(w, finalURL)
}
