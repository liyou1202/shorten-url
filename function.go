package shorten_url

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/gin-gonic/gin"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
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

//	PostResp{
//		isSuccess: "true",
//		shortenStr: "a1b2c3",
//		message: ""
//	}
type PostRespBody struct {
	IsSuccess  string `json:"isSuccess"`
	ShortenStr string `json:"shortenStr"`
	Message    string `json:"message"`
}

const (
	BucketName        = "shorten.liyou-chen.com"
	ShortenRecordName = "shortenRecord.json"
)

var ctx context.Context
var client *storage.Client

var respBody = &PostRespBody{
	IsSuccess:  "false",
	ShortenStr: "",
	Message:    "",
}

func init() {
	functions.HTTP("RequestHandler", RequestHandler)
}

func RequestHandler(w http.ResponseWriter, r *http.Request) {
	// Connect with GCP services
	ctx = context.Background()
	var err error

	client, err = storage.NewClient(ctx)
	if err != nil {
		respBody.Message = fmt.Sprintf("Could not get context or connect to client: %s", err)
		marshaledJson, _ := json.Marshal(respBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(marshaledJson)
		return
	}
	defer client.Close()

	router := gin.Default()
	// Set CORS headers for request.
	router.Use(CORSMiddleware())
	router.POST("/shorten", shortenHandler)
	router.GET("/:shorten", redirectHandler)
	router.ServeHTTP(w, r)
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func redirectHandler(c *gin.Context) {
	shortenString := c.Param("shorten")
	redirectUrl, err := getOriginUrlByShorten(shortenString)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}
	c.Redirect(http.StatusSeeOther, redirectUrl)
}

func shortenHandler(c *gin.Context) {
	fmt.Println(c.Request.Body)
	shortened, err := shortenUrl(c.Request.Body)
	if err != nil {
		respBody.Message = err.Error()
		c.JSON(http.StatusBadRequest, respBody)
		return
	}
	respBody.IsSuccess = "true"
	respBody.ShortenStr = shortened
	c.JSON(http.StatusOK, respBody)
}

func shortenUrl(reqBody io.ReadCloser) (string, error) {
	var data PostData
	decoder := json.NewDecoder(reqBody)
	err := decoder.Decode(&data)
	if err != nil {
		return "", errors.New("failed to decode post body")
	}
	defer reqBody.Close()

	longUrl := url.URL{
		Scheme: data.Scheme,
		Host:   data.Domain,
		Path:   data.Path,
	}

	shortenString, err := queryShortenFromRecord(longUrl.String())
	if err != nil {
		return "", errors.New("failed to parse record")
	}

	if shortenString != "" {
		return shortenString, nil
	}

	shortenString = generateRandomShortenString()
	newRecord := &ShortenedRecord{
		Origin:     longUrl.String(),
		Shorten:    shortenString,
		CreateTime: strconv.Itoa(int(time.Now().Unix())),
	}

	err = addToRecord(newRecord)
	if err != nil {
		return "", errors.New("failed to add new data to record")
	}

	return shortenString, nil
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

func queryShortenFromRecord(queryString string) (string, error) {
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

func getOriginUrlByShorten(shorten string) (string, error) {
	var records []ShortenedRecord
	data, err := readJsonFromBucket(records, BucketName, ShortenRecordName)
	if err != nil {
		return "", err
	}

	for _, v := range data {
		if v.Shorten == shorten {
			return v.Origin, nil
		}
	}
	return "", errors.New("Corresponding origin URL could not be found ")
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

func responseFormattedWriter(w http.ResponseWriter, body PostRespBody, statusCode int) {
	json, _ := json.Marshal(body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(json)
}
