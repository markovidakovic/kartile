package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

type Activity struct {
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	http.HandleFunc("/activities", activitiesHandler)

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", nil)
}

func activitiesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getActivities(w, r)
	}
}

func getActivities(w http.ResponseWriter, r *http.Request) {
	var act []Activity = []Activity{
		{Id: 1, Title: "activity 1", TypeId: 1},
		{Id: 2, Title: "activity 2", TypeId: 2},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(act)
}
