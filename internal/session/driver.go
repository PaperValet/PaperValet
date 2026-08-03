package session

// 空白导入 pure-Go SQLite 驱动，把 `sqlite` 注册到 database/sql。
// 不引入 cgo，匹配 README 中 "no CGO" 的承诺；与 mattn/go-sqlite3 二选一。
//
// modernc.org/sqlite 注册的 driver 名固定为 "sqlite"，与
// internal/session/manager.go 中 sql.Open("sqlite", ...) 对应。
//
// 版本由 go.mod / go.sum 锁定；升级时执行：
//
//	go get modernc.org/sqlite@latest
//	go mod tidy
import _ "modernc.org/sqlite"
