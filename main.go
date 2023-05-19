package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

type activity struct {
	Id     int    `json:"id"`
	Title  string `json:"title"`
	TypeId int    `json:"type_id"`
}

func main() {
	connStr := "postgresql://kartile:your_password@localhost:5432/kartile?schema=public"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", nil)
}
