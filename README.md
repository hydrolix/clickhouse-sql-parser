# clickhouse-sql-parser

A Go library for parsing [ClickHouse SQL](https://clickhouse.com/docs/en/sql-reference/) statements.

> 🛠️ This is a maintained fork of [`AfterShip/clickhouse-sql-parser`](https://github.com/AfterShip/clickhouse-sql-parser), with improvements and customizations by [Hydrolix](https://github.com/hydrolix).

## ✨ Features

- Converts ClickHouse SQL into an abstract syntax tree (AST)
- Lightweight and easy to embed in Go applications

## 📦 How to use
You can use it as your Go library
```bash
go get github.com/hydrolix/clickhouse-sql-parser@latest
```

```go
import "github.com/hydrolix/clickhouse-sql-parser/parser"

input := "SELECT count() FROM events"

ast, err := parser.Parse(input)
if err != nil {
log.Fatalf("parse error: %v", err)
}

// Use AST...
```

## 📄 License
Licensed under the MIT License, same as the original project.

## 🙏 Credits
Originally created by AfterShip.
Forked and maintained by Hydrolix.