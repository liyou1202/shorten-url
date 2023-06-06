package shorten_url

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"time"
)

type StaticFileInfo struct {
	path        string
	name        string
	contentType string
}

//	ShortenedRecord {
//	   "origin": "https://liyou-chen.site/#skills",
//	   "shorten": "abcd",
//	   "createTime": "1686023936"
//	}
type ShortenedRecord struct {
	Origin     string `json:"origin"`
	Shorten    string `json:"shorten"`
	CreateTime string `json:"createTime"`
}

// PostData {
// "scheme": "https",
// "domain": "liyou-chen.site",
// "path": "#skills",
// }
type PostData struct {
	Scheme string `json:"scheme"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

const (
	BucketName        = "shorten-url-static-bucket"
	StaticsFilePath   = "statics"
	HtmlName          = "index.html"
	ShortenRecordName = "shortenRecord.json"
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

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	var data PostData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("POST body decode failed"))
		return
	}
	defer r.Body.Close()
	longUrl := url.URL{
		Scheme: data.Scheme,
		Host:   data.Domain,
		Path:   data.Path,
	}

	shortenString, err := getRecordShorten(longUrl.String())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Record parsing failed"))
		return
	}

	if shortenString != "" {
		//TODO Return shorten URL using json formatted
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(shortenString))
		return
	}

	shortenString = generateRandomShortenString()
	newRecord := &ShortenedRecord{
		Origin:     longUrl.String(),
		Shorten:    shortenString,
		CreateTime: time.Now().UTC().String(),
	}

	err = addToRecord(newRecord)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Failed to add new data to record"))
		return
	}

	//TODO Return shorten URL using json formatted
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(shortenString))
}

func addToRecord(newData *ShortenedRecord) error {
	var records []ShortenedRecord
	data, err := readJsonFromBucket(records, BucketName, ShortenRecordName)
	if err != nil {
		return err
	}
	data = append(data, *newData)

	jsonByte, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return writeToBucket(jsonByte, BucketName, ShortenRecordName)
}

func generateRandomShortenString() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	//TODO Check random shortened string is unique
	return string(b)
}

func getRecordShorten(queryString string) (string, error) {
	var records []ShortenedRecord
	data, err := readJsonFromBucket(records, BucketName, ShortenRecordName)
	if err != nil {
		return "", err
	}

	for _, v := range data {
		if v.Origin == queryString {
			return v.Shorten, nil
		}
	}
	return "", nil
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

func readJsonFromBucket[T any](container T, bucketName, path string) (T, error) {
	bkt := client.Bucket(bucketName)
	obj := bkt.Object(path)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return container, fmt.Errorf("Failed to read object: %v ", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return container, fmt.Errorf("Failed to read data: %v ", err)
	}
	fmt.Sprintf("bucket is: %s \n path is: %s \n data is: %s", BucketName, path, string(data))
	err = json.Unmarshal(data, &container)
	if err != nil {
		return container, fmt.Errorf("Failed to parse JSON: %v ", err)
	}

	return container, nil
}

func writeToBucket(data []byte, bucketName, path string) error {
	bkt := client.Bucket(bucketName)
	obj := bkt.Object(path)

	w := obj.NewWriter(ctx)
	w.ContentType = "application/json"
	w.CacheControl = "no-cache"

	_, err := w.Write(data)
	if err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return nil
}
