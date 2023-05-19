package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

type route struct {
	handler http.HandlerFunc
}

type matchedRoute struct {
	route *route
}

type router struct {
	routes []*route
}

func newRouter() *router {
	return &router{
		routes: make([]*route, 0),
	}
}

func (rtr *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	http.NotFound(w, r)
}

func (rtr *router) handleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	rt := &route{
		handler: handler,
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

	r.handleFunc("/activities/{activityId}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", r)
}
