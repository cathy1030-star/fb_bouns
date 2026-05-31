package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
)

// Record 定義輸入的項目結構
type Record struct {
	Nickname string `json:"a"` // 得獎暱稱
	AccID    string `json:"b"` // accid
	GameName string `json:"c"` // 遊戲暱稱
}

// StatsResult 用於回傳統計結果
type StatsResult struct {
	Record
	Count int `json:"count"`
}

var (
	// 用來儲存統計資料，Key 是 Record 結構
	stats   = make(map[Record]int)
	statsMu sync.RWMutex
	tmpl    = template.Must(template.ParseFiles("index.html"))
)

func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/api/bulk-add", handleBulkAdd)
	http.HandleFunc("/api/stats", handleGetStats)

	fmt.Println("伺服器已啟動於 http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

// 顯示首頁
func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl.Execute(w, nil)
}

// 處理新增資料
func handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只允許 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	var rec Record
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if rec.Nickname == "" || rec.AccID == "" || rec.GameName == "" {
		http.Error(w, "欄位不可為空", http.StatusBadRequest)
		return
	}

	// 更新統計次數
	statsMu.Lock()
	stats[rec]++
	statsMu.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "success")
}

// 處理批量新增資料
func handleBulkAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只允許 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	var recs []Record
	if err := json.NewDecoder(r.Body).Decode(&recs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	statsMu.Lock()
	for _, rec := range recs {
		if rec.Nickname != "" && rec.AccID != "" && rec.GameName != "" {
			stats[rec]++
		}
	}
	statsMu.Unlock()

	w.WriteHeader(http.StatusOK)
}

// 取得目前的統計清單
func handleGetStats(w http.ResponseWriter, r *http.Request) {
	statsMu.RLock()
	defer statsMu.RUnlock()

	var results []StatsResult
	for rec, count := range stats {
		results = append(results, StatsResult{
			Record: rec,
			Count:  count,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
