package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
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

	// 用來儲存 accid+gamename -> nickname 的對應關係
	registry   = make(map[string]string)
	registryMu sync.RWMutex

	tmpl = template.Must(template.ParseFiles("index.html"))
)

func main() {
	loadData() // 啟動時讀取現有資料

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/api/bulk-add", handleBulkAdd)
	http.HandleFunc("/api/stats", handleGetStats)
	http.HandleFunc("/api/registry", handleGetRegistry)
	http.HandleFunc("/api/lookup-nicknames", handleLookupNicknames)

	fmt.Println("伺服器已啟動於 http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

// 將資料儲存至 data.json
func saveData() {
	statsMu.RLock()
	registryMu.RLock()
	defer statsMu.RUnlock()
	defer registryMu.RUnlock()

	var data struct {
		Stats    []StatsResult     `json:"stats"`
		Registry map[string]string `json:"registry"`
	}
	for rec, count := range stats {
		data.Stats = append(data.Stats, StatsResult{Record: rec, Count: count})
	}
	data.Registry = registry

	b, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile("data.json", b, 0644)
}

// 從 data.json 讀取資料
func loadData() {
	b, err := os.ReadFile("data.json")
	if err != nil {
		return // 若檔案不存在則跳過
	}

	var data struct {
		Stats    []StatsResult     `json:"stats"`
		Registry map[string]string `json:"registry"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return
	}

	for _, s := range data.Stats {
		stats[s.Record] = s.Count
	}
	registry = data.Registry
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

	if rec.AccID == "" || rec.GameName == "" {
		http.Error(w, "AccID 與 遊戲暱稱不可為空", http.StatusBadRequest)
		return
	}

	regKey := rec.AccID + "_" + rec.GameName

	registryMu.Lock()
	if rec.Nickname != "" {
		// 如果有輸入暱稱，更新資料庫
		registry[regKey] = rec.Nickname
	} else {
		// 如果沒輸入，嘗試從資料庫找
		if name, ok := registry[regKey]; ok {
			rec.Nickname = name
		} else {
			registryMu.Unlock()
			http.Error(w, "此 ID 為新資料，請至少輸入一次『得獎暱稱』以供系統記憶。", http.StatusBadRequest)
			return
		}
	}
	registryMu.Unlock()

	// 更新統計次數
	statsMu.Lock()
	stats[rec]++
	statsMu.Unlock()

	saveData() // 儲存變更
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

	var addedCount int
	registryMu.Lock()
	statsMu.Lock()
	for i := range recs {
		rec := &recs[i] // 使用指標直接修改原物件
		if rec.AccID != "" && rec.GameName != "" {
			regKey := rec.AccID + "_" + rec.GameName
			if rec.Nickname != "" {
				// 優先使用輸入的暱稱並更新資料庫
				registry[regKey] = rec.Nickname
			}

			// 如果沒輸入暱稱，嘗試從資料庫補齊
			if rec.Nickname == "" {
				if name, ok := registry[regKey]; ok {
					rec.Nickname = name
				}
			}

			// 只有在暱稱存在的情況下才記錄統計
			if rec.Nickname != "" {
				key := Record{Nickname: rec.Nickname, AccID: rec.AccID, GameName: rec.GameName}
				stats[key]++
				addedCount++
			}
		}
	}
	statsMu.Unlock()
	registryMu.Unlock()

	saveData() // 儲存變更
	if addedCount == 0 && len(recs) > 0 {
		http.Error(w, "提交失敗：貼上的名單皆為新 ID 且未包含暱稱，請在名單中至少包含一次『暱稱』欄位。", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "成功記錄 %d 筆資料！", addedCount)
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

// 取得暱稱資料庫清單
func handleGetRegistry(w http.ResponseWriter, r *http.Request) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var list []struct {
		Key      string `json:"key"`
		Nickname string `json:"nickname"`
	}
	for k, v := range registry {
		list = append(list, struct {
			Key      string `json:"key"`
			Nickname string `json:"nickname"`
		}{Key: k, Nickname: v})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// 查詢指定暱稱清單的總次數
func handleLookupNicknames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只允許 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	var names []string
	if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	statsMu.RLock()
	defer statsMu.RUnlock()

	results := make(map[string]int)
	for _, name := range names {
		results[name] = 0 // 預設 0 次
		for rec, count := range stats {
			if rec.Nickname == name {
				results[name] += count
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
