package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "github.com/lib/pq"
)

type route struct {
	handler http.HandlerFunc
	pattern string
}

func (rt *route) match(patternParts []string, incomingParts []string) bool {
	var isMatch bool = false
	if len(patternParts) != len(incomingParts) {
		isMatch = false
		return isMatch
	}

	// loop pattern parts and compare it to incoming parts and check if a part is a parameter
	for i := 0; i < len(patternParts); i++ {
		if patternParts[i] != incomingParts[i] && !isParameter(patternParts[i]) {
			isMatch = false
			break
		} else {
			isMatch = true
		}
	}

	return isMatch
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
	incomingPath := r.URL.Path
	incomingPathParts := strings.Split(incomingPath, "/")

	var handler http.Handler
	handler = http.NotFoundHandler()

	for _, rt := range rtr.routes {
		patternParts := strings.Split(rt.pattern, "/")
		if isMatch := rt.match(patternParts, incomingPathParts); isMatch {
			handler = rt.handler
		}
	}

	handler.ServeHTTP(w, r)
}

func (rtr *router) handleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	rt := &route{
		handler: handler,
		pattern: pattern,
	}
	rtr.routes = append(rtr.routes, rt)
}

func isParameter(part string) bool {
	return strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
}

func braceIndices(s string) ([]int, error) {
	var level, idx int
	var idxs []int
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if level++; level == 1 {
				idx = i
			}
		case '}':
			if level--; level == 0 {
				idxs = append(idxs, idx, i+1)
			} else if level < 0 {
				return nil, fmt.Errorf("unbalanced braces in %q", s)
			}
		}
	}
	if level != 0 {
		return nil, fmt.Errorf("unbalanced brances in %q", s)
	}
	return idxs, nil
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
	r.handleFunc("/activities", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("activities"))
	})
	r.handleFunc("/activities/{activityId}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("activities by id"))
	})

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", r)
}
