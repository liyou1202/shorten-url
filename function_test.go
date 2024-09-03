package shorten_url

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"testing"
	"time"
)

func TestShortenUrl(t *testing.T) {
	// 啟動外部進程
	cmd := exec.Command("go", "run", "cmd/main.go")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// 確保進程在測試結束後能夠關閉
	defer func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Errorf("Failed to kill process: %v", err)
		}
	}()

	// 等待一段時間以確保進程啟動
	time.Sleep(2 * time.Second)
	testUrl := "http://localhost:8080/shorten"
	testData := &PostData{
		Scheme: "https",
		Domain: "www.linkedin.com",
		Path:   "/in/liyou-chen-435511251",
	}

	// 將數據編碼為 JSON
	jsonData, err := json.Marshal(testData)
	if err != nil {
		fmt.Printf("Failed to encode data to JSON: %v\n", err)
		return
	}

	resp, err := http.Post(testUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Failed to send request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response body: %v\n", err)
		return
	}

	expectedBody := `{"isSuccess":"true","shortenStr":"caxnw9","message":""}`
	if !bytes.Equal(body, []byte(expectedBody)) {
		t.Errorf("Unexpected response body: got %s, want %s", body, expectedBody)
	}

	resp, err = http.Get("https://redirect.liyou-chen.com/caxnw9")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()
	expectedURL := "https://www.linkedin.com/in/liyou-chen-435511251"
	if resp.Request.URL.String() != expectedURL {
		t.Fatalf("Wrong direction: %v", err)
	}
}
