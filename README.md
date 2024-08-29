
# Ignite

<img src="./image.png" alt="Header" width="200" /> 

**Ignite** 是一個通用的物件池管理庫，旨在為 Go 語言開發者提供高效的資源管理解決方案。通過該庫，可以避免過度創建和銷毀 struct，從而顯著減少 GC 壓力並提升應用程式的效能和穩定性。這個庫專為需要高效併發和靈活資源管理的應用場景設計，適用於各種資料庫連接池、物件池或任何需要頻繁創建和釋放資源的場景。

## 功能介紹 (Features)

- **通用性**：使用 Go 泛型 (`T any`) 實現，可以適應不同類型的物件和資源。
- **高效併發**：通過 `sync.Pool` 和 `atomic`，支援高效的物件回收和併發操作，有效避免了鎖競爭。
- **靈活配置**：使用 `Config[T]` 結構體提供豐富的配置選項，如初始大小、最大最小池大小、閒置時間、健康檢查等。
- **減少 GC 負擔**：透過有效的物件重用機制，顯著減少了內存分配和垃圾回收次數，降低了 GC 的工作負載。
- **健康檢查與清理**：內建物件健康檢查和清理功能，確保池中的物件始終處於可用狀態，防止使用無效或損壞的物件。
- **輕量化依賴**：依賴最小化，易於集成和部署。
- **易於擴展**：提供簡單且直觀的 API，開發者可以根據特定需求進一步擴展或自定義物件池的行為。

## 安裝 (Installation)

要安裝此庫，可以使用以下命令：

```bash
go get goflare.io/ignite
```

## 使用指南 (Usage Guide)

### 概述

`Ignite` 提供了一種簡單高效的方法來管理各種物件的池化和重用。以下示例將展示如何使用 `Ignite` 來管理自定義的 `User` 結構體。

### 定義 User 結構體

首先，讓我們定義一個 `User` 結構體，這是我們要管理的資源類型。

```go
type User struct {
    ID    int
    Name  string
    Email string
}
```

### 創建物件池

要創建物件池，您需要設置配置並調用 `NewPool` 方法。

```go
package main

import (
	"context"
	"fmt"
	"goflare.io/ignite"
	"time"
)

func main() {
	// 定義物件池配置
	config := ignite.Config[*User]{
		InitialSize: 5,                // 初始池大小
		MaxSize:     20,               // 最大池大小
		MinSize:     2,                // 最小池大小
		MaxIdleTime: 10 * time.Minute, // 最大閒置時間
		Factory: func() (*User, error) {
			return &User{}, nil // 初始化 User 結構
		},
		Reset: func(user *User) error {
			// 重置 User 結構（例如清空數據）
			user.ID = 0
			user.Name = ""
			user.Email = ""
			return nil
		},
		Validate: func(user *User) error {
			// 檢查 User 結構是否有效
			if user.ID == 0 {
				return fmt.Errorf("invalid user ID")
			}
			return nil
		},
	}

	// 創建物件池
	pool, err := ignite.NewPool(config)
	if err != nil {
		fmt.Println("Failed to create pool:", err)
		return
	}

	// 從池中獲取物件
	ctx := context.Background()
	userWrapper, err := pool.Get(ctx)
	if err != nil {
		fmt.Println("Failed to get user from pool:", err)
		return
	}

	// 使用 User 結構
	user := userWrapper.Object
	user.ID = 1
	user.Name = "John Doe"
	user.Email = "john.doe@example.com"
	fmt.Printf("User: %+v
	", user)

	// 歸還 User 結構到池中
	pool.Put(userWrapper)

	// 關閉池
	err = pool.Close(ctx)
	if err != nil {
		fmt.Println("Failed to close pool:", err)
	}
}
```

### 進階使用場景

#### 健康檢查與自動清理

Ignite 支援自動健康檢查與清理功能，可以避免使用損壞或無效的物件。

- **健康檢查 (Health Check)**: 自動定期檢查池中的物件狀態，確保其可用性。
- **自動清理 (Auto Cleanup)**: 根據配置的閒置時間自動清理閒置時間過長的物件。

這些功能可以通過配置 `HealthCheck` 和 `MaxIdleTime` 等參數進行調整。

#### 動態調整池大小

池的大小可以動態調整，這樣可以根據當前的負載和需求進行調整，提高資源使用效率。可以使用 `Resize(newSize int)` 方法來調整池的大小。

#### 進一步擴展

通過修改配置或擴展現有的 `Config` 和 `Pool` 接口，開發者可以定制池的行為。例如，可以增加新的健康檢查機制、定制物件的創建和銷毀邏輯等。

## Contributing

We welcome all forms of contribution! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for more information.

## License

Ember is distributed under the MIT License. For more details, see [LICENSE](LICENSE).
