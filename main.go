package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

type route struct {
	handler http.HandlerFunc
	title   string
	path    string
}

type router struct {
	routes []*route
}

func newRouter() *router {
	return &router{}
}

func (rtr *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, v := range rtr.routes {
		if v.path == r.URL.Path {
			v.handler(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func (rtr *router) handleFunc(path string, handler http.HandlerFunc) {
	rt := &route{
		handler: handler,
		path:    path,
	}
	rtr.routes = append(rtr.routes, rt)
}

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

	r := newRouter()

	r.handleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", r)
}

func activitiesHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	switch r.Method {
	case "GET":
		getActivities(w, r)
	}
}

func getActivities(w http.ResponseWriter, r *http.Request) {
	var act []activity = []activity{
		{Id: 1, Title: "activity 1", TypeId: 1},
		{Id: 2, Title: "activity 2", TypeId: 2},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(act)
}
