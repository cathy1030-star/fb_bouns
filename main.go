package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"sync"
)

// Winner 儲存得獎者名稱、得獎次數等詳細資訊
type Winner struct {
	Name     string
	AccID    string
	GameName string
	Count    int
}

var (
	// mu 確保並發情況下對 map 進行讀寫是安全的
	mu sync.Mutex
	// records 儲存得獎暱稱對應的詳細資料與次數
	records = make(map[string]*Winner)
)

// htmlTemplate 是網頁的畫面模板，包含輸入表單與排行榜
const htmlTemplate = `
<!DOCTYPE html>
<html lang="zh-TW">
<head>
	<meta charset="UTF-8">
	<title>活動名單整理系統</title>
	<style>
		body { font-family: "微軟正黑體", sans-serif; margin: 40px; background-color: #f9f9f9; }
		.container { max-width: 600px; margin: auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 4px 8px rgba(0,0,0,0.1); }
		.form-group { margin-bottom: 15px; }
		label { display: inline-block; width: 120px; font-weight: bold; }
		input[type="text"] { width: 250px; padding: 8px; border: 1px solid #ccc; border-radius: 4px; }
		button { padding: 10px 20px; background-color: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
		button:hover { background-color: #0056b3; }
		table { width: 100%; border-collapse: collapse; margin-top: 20px; }
		th, td { border: 1px solid #ddd; padding: 10px; text-align: left; }
		th { background-color: #007bff; color: white; }
	</style>
</head>
<body>
	<div class="container">
		<h2>📝 新增得獎資料</h2>
		<form method="POST" action="/">
			<div class="form-group">
				<label>a: 得獎暱稱</label>
				<input type="text" name="nickname" required placeholder="必填">
			</div>
			<div class="form-group">
				<label>b: accid</label>
				<input type="text" name="accid">
			</div>
			<div class="form-group">
				<label>c: 遊戲暱稱</label>
				<input type="text" name="gamename">
			</div>
			<button type="submit">送出新增</button>
		</form>

		<hr style="margin: 30px 0;">

		<h2>🏆 得獎次數排行榜 🏆</h2>
		<table>
			<tr>
				<th>得獎暱稱</th>
				<th>accid</th>
				<th>遊戲暱稱</th>
				<th>得獎次數</th>
			</tr>
			{{range .}}
			<tr>
				<td>{{.Name}}</td>
				<td>{{.AccID}}</td>
				<td>{{.GameName}}</td>
				<td>{{.Count}}</td>
			</tr>
			{{end}}
		</table>
	</div>
</body>
</html>
`

func handler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// 如果是 POST 請求，代表使用者送出了表單
	if r.Method == http.MethodPost {
		r.ParseForm()
		nickname := r.FormValue("nickname")
		accid := r.FormValue("accid")
		gamename := r.FormValue("gamename")

		// 只要有輸入得獎暱稱，就進行紀錄與統計
		if nickname != "" {
			// 檢查是否已經有這位得獎者
			if w, exists := records[nickname]; exists {
				w.Count++
				// 若後續輸入有提供新的 accid 或遊戲暱稱，則更新它
				if accid != "" {
					w.AccID = accid
				}
				if gamename != "" {
					w.GameName = gamename
				}
			} else {
				// 新增一位得獎者資料
				records[nickname] = &Winner{
					Name:     nickname,
					AccID:    accid,
					GameName: gamename,
					Count:    1,
				}
			}
		}

		// 新增完畢後重新導向回首頁，避免重新整理時重複送出表單
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 如果是 GET 請求，處理統計排序並渲染網頁
	var winners []Winner
	for _, w := range records {
		winners = append(winners, *w)
	}

	sort.Slice(winners, func(i, j int) bool {
		return winners[i].Count > winners[j].Count
	})

	// 解析並輸出 HTML 網頁
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	tmpl.Execute(w, winners)
}

func main() {
	// 設定網頁的路由與處理函式
	http.HandleFunc("/", handler)

	fmt.Println("伺服器已啟動！請開啟瀏覽器並前往: http://localhost:8080")

	// 啟動伺服器在 8080 port
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("伺服器啟動失敗:", err)
	}
}
