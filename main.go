package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "github.com/lib/pq"
)

type contextKey int

const (
	paramsKey contextKey = iota
)

type route struct {
	handler http.HandlerFunc
	pattern string
}

func (rt *route) match(patternParts []string, incomingParts []string, matchedRoute *matchedRoute) bool {
	var isMatch bool = false
	if len(patternParts) != len(incomingParts) {
		isMatch = false
		return isMatch
	}

	// loop pattern parts and compare it to incoming parts and also check if a part is a parameter
	for i := 0; i < len(patternParts); i++ {
		if patternParts[i] != incomingParts[i] && !isParameter(patternParts[i], incomingParts[i], matchedRoute) {
			isMatch = false
			break
		} else {
			isMatch = true
		}
	}

	return isMatch
}

type matchedRoute struct {
	route  *route
	params map[string]string
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
	var matchedRoute matchedRoute = matchedRoute{
		params: make(map[string]string),
	}
	handler = http.NotFoundHandler()

	for _, rt := range rtr.routes {
		patternParts := strings.Split(rt.pattern, "/")
		if isMatch := rt.match(patternParts, incomingPathParts, &matchedRoute); isMatch {
			handler = rt.handler
			r = requestWithParams(r, matchedRoute.params)
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

func params(r *http.Request) map[string]string {
	if rp := r.Context().Value(paramsKey); rp != nil {
		return rp.(map[string]string)
	}
	return nil
}

func requestWithParams(r *http.Request, params map[string]string) *http.Request {
	ctx := context.WithValue(r.Context(), paramsKey, params)
	return r.WithContext(ctx)
}

func isParameter(pp string, ip string, matchedRoute *matchedRoute) bool {
	if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
		idxs, err := braceIndices(pp)
		if err != nil {
			fmt.Println(err)
		}
		paramTitle := pp[idxs[0]+1 : idxs[1]-1]
		matchedRoute.params[paramTitle] = ip
		return true
	}
	return false
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

type server struct {
	db *sql.DB
}

var srvr server

type activityType struct {
	Id    int    `json:"id"`
	Title string `json:"title"`
}

type activity struct {
	Id     int    `json:"id"`
	Title  string `json:"title"`
	TypeId int    `json:"type_id"`
}

func main() {
	connStr := "postgresql://kartile:your_password@localhost:5432/kartile"
	db, err := sql.Open("postgres", connStr)
	srvr.db = db
	if err != nil {
		log.Fatal(err)
	}
	defer srvr.db.Close()

	r := newRouter()

	r.handleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	r.handleFunc("/activities", handleActivities)
	r.handleFunc("/activities/{activityId}", handleActivitiesById)
	r.handleFunc("/activities/types", handleActivityTypes)
	r.handleFunc("/activities/types/{typeId}", handleActivityTypesById)

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", r)
}

func handleActivityTypes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		at, err := getActivityTypes(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(at)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	case "POST":
		at, err := createActivityType(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(at)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func getActivityTypes(w http.ResponseWriter, r *http.Request) ([]activityType, error) {
	rows, err := srvr.db.Query("SELECT id, title FROM activity_types")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actts []activityType
	for rows.Next() {
		var actt activityType
		if err := rows.Scan(&actt.Id, &actt.Title); err != nil {
			return nil, err
		}
		actts = append(actts, actt)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return actts, nil
}

func createActivityType(w http.ResponseWriter, r *http.Request) (activityType, error) {
	type request struct {
		Title string `json:"title"`
	}
	var data request
	var at activityType
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return activityType{}, err
	}
	srvr.db.QueryRow("INSERT INTO activity_types (title) VALUES ($1) RETURNING id, title", data.Title).Scan(&at.Id, &at.Title)
	return at, nil
}

func handleActivityTypesById(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		at, err := getActivityTypeById(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(at)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func getActivityTypeById(w http.ResponseWriter, r *http.Request) (activityType, error) {
	params := params(r)

	var at activityType
	err := srvr.db.QueryRow("SELECT id, title FROM activity_types WHERE id = $1", params["typeId"]).Scan(&at.Id, &at.Title)
	if err != nil {
		return activityType{}, nil
	}

	return at, nil
}

func handleActivities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		acts, err := getActivities(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(acts)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	case "POST":
		act, err := createActivity(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(act)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(resp)
	}
}

func getActivities(w http.ResponseWriter, r *http.Request) ([]activity, error) {
	rows, err := srvr.db.Query("SELECT id, title, type_id FROM activities")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acts []activity
	for rows.Next() {
		var act activity
		if err := rows.Scan(&act.Id, &act.Title, &act.TypeId); err != nil {
			return nil, err
		}
		acts = append(acts, act)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return acts, nil
}

func createActivity(w http.ResponseWriter, r *http.Request) (activity, error) {
	type request struct {
		Title  string `json:"title"`
		TypeId int    `json:"type_id"`
	}
	var data request
	var act activity
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return activity{}, err
	}
	err = srvr.db.QueryRow("INSERT INTO activities (title, type_id) VALUES ($1, $2) RETURNING id, title, type_id", data.Title, data.TypeId).Scan(&act.Id, &act.Title, &act.TypeId)
	if err != nil {
		return activity{}, err
	}
	return act, nil
}

func handleActivitiesById(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		act, err := getActivityById(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(act)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func getActivityById(w http.ResponseWriter, r *http.Request) (activity, error) {
	params := params(r)
	var act activity
	err := srvr.db.QueryRow("SELECT id, title, type_id FROM activities WHERE id = $1", params["activityId"]).Scan(&act.Id, &act.Title, &act.TypeId)
	if err != nil {
		return activity{}, err
	}
	return act, nil
}
