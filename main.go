package main

import (
    "fmt"
)

func main() {
    url := "https://meu-bucket-exemplo.s3.amazonaws.com/"

    result := CheckBucket(url)

    if result.Err != nil {
        fmt.Println("Erro:", result.Err)
        return
    }

    fmt.Println("Existe:", result.Exists)
    fmt.Println("Publico:", result.Public)
    fmt.Println("Status:", result.StatusCode)
}