
# Ignite

---

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

## 貢獻 (Contributing)

歡迎任何形式的貢獻！請參閱 [CONTRIBUTING.md](CONTRIBUTING.md) 了解更多信息。

## 授權 (License)

Ignite 根據 MIT 許可證分發。詳細信息請參閱 [LICENSE](LICENSE)。

---

**Ignite** is a general-purpose object pool management library designed to provide efficient resource management solutions for Go developers. With this library, you can avoid excessive creation and destruction of structs, significantly reducing GC pressure and enhancing the performance and stability of applications. It is designed for scenarios that require efficient concurrency and flexible resource management, suitable for various database connection pools, object pools, or any scenario that requires frequent creation and release of resources.

## Features

- **Generality**: Implemented using Go generics (`T any`), adaptable to different types of objects and resources.
- **Efficient Concurrency**: Supports efficient object recycling and concurrent operations through `sync.Pool` and `atomic`, effectively avoiding lock contention.
- **Flexible Configuration**: Provides rich configuration options through the `Config[T]` struct, such as initial size, maximum and minimum pool sizes, idle time, health checks, etc.
- **Reduced GC Burden**: Significantly reduces memory allocations and garbage collection frequency through effective object reuse, lowering the workload of GC.
- **Health Check and Cleanup**: Built-in object health check and cleanup functions ensure that objects in the pool are always in a usable state, preventing the use of invalid or damaged objects.
- **Lightweight Dependencies**: Minimal dependencies, easy to integrate and deploy.
- **Easy to Extend**: Provides a simple and intuitive API, allowing developers to further extend or customize the behavior of the object pool based on specific needs.

## Installation

To install this library, use the following command:

```bash
go get goflare.io/ignite
```

## Usage Guide

### Overview

`Ignite` provides a simple and efficient way to manage pooling and reuse of various objects. The following example demonstrates how to use `Ignite` to manage a custom `User` struct.

### Define the User Struct

First, let's define a `User` struct, which is the type of resource we want to manage.

```go
type User struct {
    ID    int
    Name  string
    Email string
}
```

### Create an Object Pool

To create an object pool, you need to set up the configuration and call the `NewPool` method.

```go
package main

import (
    "context"
    "fmt"
    "goflare.io/ignite"
    "time"
)

func main() {
    // Define object pool configuration
    config := ignite.Config[*User]{
        InitialSize: 5,                // Initial pool size
        MaxSize:     20,               // Maximum pool size
        MinSize:     2,                // Minimum pool size
        MaxIdleTime: 10 * time.Minute, // Maximum idle time
        Factory: func() (*User, error) {
            return &User{}, nil // Initialize User struct
        },
        Reset: func(user *User) error {
            // Reset User struct (e.g., clear data)
            user.ID = 0
            user.Name = ""
            user.Email = ""
            return nil
        },
        Validate: func(user *User) error {
            // Check if the User struct is valid
            if user.ID == 0 {
                return fmt.Errorf("invalid user ID")
            }
            return nil
        },
    }

    // Create the object pool
    pool, err := ignite.NewPool(config)
    if err != nil {
        fmt.Println("Failed to create pool:", err)
        return
    }

    // Get an object from the pool
    ctx := context.Background()
    userWrapper, err := pool.Get(ctx)
    if err != nil {
        fmt.Println("Failed to get user from pool:", err)
        return
    }

    // Use the User struct
    user := userWrapper.Object
    user.ID = 1
    user.Name = "John Doe"
    user.Email = "john.doe@example.com"
    fmt.Printf("User: %+v
", user)

    // Return the User struct to the pool
    pool.Put(userWrapper)

    // Close the pool
    err = pool.Close(ctx)
    if err != nil {
        fmt.Println("Failed to close pool:", err)
    }
}
```

### Advanced Usage Scenarios

#### Health Checks and Auto Cleanup

Ignite supports automatic health checks and cleanup functions to avoid using damaged or invalid objects.

- **Health Check**: Automatically performs regular checks on the objects in the pool to ensure their availability.
- **Auto Cleanup**: Automatically cleans up objects that have been idle for too long based on the configured idle time.

These features can be adjusted by configuring parameters such as `HealthCheck` and `MaxIdleTime`.

#### Dynamic Pool Size Adjustment

The size of the pool can be adjusted dynamically, allowing for changes based on current load and demand to improve resource usage efficiency. You can use the `Resize(newSize int)` method to adjust the pool size.

#### Further Extension

Developers can customize the behavior of the pool by modifying configurations or extending existing `Config` and `Pool` interfaces. For example, new health check mechanisms can be added, or custom object creation and destruction logic can be implemented.

## Contributing

We welcome all forms of contribution! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for more information.

## License

Ignite is distributed under the MIT License. For more details, see [LICENSE](LICENSE).