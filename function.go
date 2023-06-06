package shorten_url

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
)

type StaticFileInfo struct {
	path        string
	name        string
	contentType string
}

const (
	BucketName      = "shorten-url-static-bucket"
	StaticsFilePath = "statics"
	HtmlName        = "index.html"
)

var ctx context.Context
var client *storage.Client

func init() {
	functions.HTTP("RequestHandler", RequestHandler)
}

func RequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx = context.Background()
	var err error

	client, err = storage.NewClient(ctx)
	if err != nil {
		msg := fmt.Sprintf("Could not get context or connect to client: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(msg))
	}
	defer client.Close()

	// Set CORS headers for request.
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Max-Age", "3600")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	p := r.URL.Path
	pattern := regexp.MustCompile("/statics/(.+.(js|css))")

	if p == "/" {
		html := StaticFileInfo{
			path:        filepath.Join(StaticsFilePath, HtmlName),
			name:        HtmlName,
			contentType: "html",
		}
		staticFileHandler(w, r, html)
		return

	} else if pattern.MatchString(p) {
		//Handle static resources e.g. js, css
		match := pattern.FindStringSubmatch(p)
		if len(match) != 3 {
			msg := fmt.Sprintf("File not found or content-type not be supported %s", p)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(msg))
		}
		fmt.Sprintf("match is : %v", match)
		fileInfo := StaticFileInfo{
			path:        filepath.Join(StaticsFilePath, match[1]),
			name:        match[1],
			contentType: match[2],
		}
		staticFileHandler(w, r, fileInfo)
		return

	} else if p == "/shorten" {
		shortenHandler(w, r)
		return
	}

	//TODO Handle url redirection action
	w.WriteHeader(http.StatusMovedPermanently)
	w.Write([]byte("connection success"))
}

// TODO Generate shorten url store to data storage and return result to API caller
func shortenHandler(w http.ResponseWriter, r *http.Request) {

}

func staticFileHandler(w http.ResponseWriter, r *http.Request, info StaticFileInfo) {
	bkt := client.Bucket(BucketName)
	obj := bkt.Object(info.path)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		msg := fmt.Sprintf("File not found %s", info.path)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(msg))
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		msg := fmt.Sprintf("Unable to read file %s", info.path)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(msg))
	}

	if info.contentType == "html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	if info.contentType == "js" {
		w.Header().Set("Content-Type", "application/javascript")
	} else if info.contentType == "css" {
		w.Header().Set("Content-Type", "text/css")
	}

	http.ServeContent(w, r, info.name, reader.Attrs.LastModified, bytes.NewReader(data))
}
