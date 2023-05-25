package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
)

type contextKey int

const (
	paramsKey contextKey = iota
	requestAccKey
)

var (
	signingKey = []byte("secret")
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

func reqAccount(r *http.Request) *account {
	if ra := r.Context().Value(requestAccKey); ra != nil {
		ra, ok := ra.(*account)
		if !ok {
			return nil
		}
		return ra
	}
	return nil
}

func requestWithParams(r *http.Request, params map[string]string) *http.Request {
	ctx := context.WithValue(r.Context(), paramsKey, params)
	return r.WithContext(ctx)
}

func requestWithAccount(r *http.Request, acc *account) *http.Request {
	ctx := context.WithValue(r.Context(), requestAccKey, acc)
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
	Id      int    `json:"id"`
	Title   string `json:"title"`
	TypeId  int    `json:"type_id"`
	OwnerId int    `json:"owner_id"`
}

type account struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	password string
}

type authAccount struct {
	account
	AccessToken string `json:"access_token"`
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
	r.handleFunc("/auth/signup", enableCorsMiddleware(handleAuth))
	r.handleFunc("/auth/tokens/access", enableCorsMiddleware(handleAuth))
	r.handleFunc("/activities", enableCorsMiddleware(accessTokenMiddleware(handleActivities)))
	r.handleFunc("/activities/{activityId}", enableCorsMiddleware(accessTokenMiddleware(handleActivitiesById)))
	r.handleFunc("/activities/types", enableCorsMiddleware(accessTokenMiddleware(handleActivityTypes)))
	r.handleFunc("/activities/types/{typeId}", enableCorsMiddleware(accessTokenMiddleware(handleActivityTypesById)))
	r.handleFunc("/accounts", enableCorsMiddleware(accessTokenMiddleware(handleAccounts)))
	r.handleFunc("/accounts/{accountId}", enableCorsMiddleware(accessTokenMiddleware(handleAccountsById)))

	fmt.Println("server running on localhost:5000")
	http.ListenAndServe(":5000", r)
}

func accessTokenMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ah := r.Header.Get("Authorization")
		if ah == "" || !strings.HasPrefix(ah, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(ah, "Bearer ")
		c, err := validateAccessToken(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var acc account
		err = srvr.db.QueryRow("SELECT id, name, email FROM accounts WHERE email = $1", c.Email).Scan(&acc.Id, &acc.Name, &acc.Email)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		r = requestWithAccount(r, &acc)

		h(w, r)
	}
}

func enableCorsMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set cors headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		h(w, r)
	}
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	var m string = r.Method
	var p string = r.URL.Path
	if m == "POST" && p == "/auth/signup" {
		a, err := createAccount(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(a)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(resp)
	} else if m == "POST" && p == "/auth/tokens/access" {
		a, err := getAuthAccount(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(a)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func createAccount(w http.ResponseWriter, r *http.Request) (authAccount, error) {
	type request struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var data request
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return authAccount{}, nil
	}
	defer r.Body.Close()
	// hash pwd
	ep, err := encryptPwd(data.Password)
	if err != nil {
		return authAccount{}, err
	}
	var resp authAccount
	srvr.db.QueryRow("INSERT INTO accounts (name, email, password) VALUES ($1, $2, $3) RETURNING id, name, email", data.Name, data.Email, ep).Scan(&resp.Id, &resp.Name, &resp.Email)
	accessToken, err := generateAccessToken(resp.Email)
	if err != nil {
		return authAccount{}, err
	}
	resp.AccessToken = accessToken
	return resp, nil
}

func getAuthAccount(w http.ResponseWriter, r *http.Request) (*authAccount, error) {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var data request
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	var acc account
	srvr.db.QueryRow("SELECT id, name, email, password FROM accounts WHERE email = $1", data.Email).Scan(&acc.Id, &acc.Name, &acc.Email, &acc.password)
	err = comparePwd(data.Password, acc.password)
	if err != nil {
		return nil, err
	}
	token, err := generateAccessToken(acc.Email)
	if err != nil {
		return nil, err
	}
	var authAcc authAccount = authAccount{
		account: account{
			Id:    acc.Id,
			Name:  acc.Name,
			Email: acc.Email,
		},
		AccessToken: token,
	}
	return &authAcc, nil
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

func createActivity(w http.ResponseWriter, r *http.Request) (*activity, error) {
	type request struct {
		Title  string `json:"title"`
		TypeId int    `json:"type_id"`
	}
	reqAcc := reqAccount(r)
	var req request
	var act activity
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return nil, err
	}
	err = srvr.db.QueryRow("INSERT INTO activities (title, type_id, owner_id) VALUES ($1, $2, $3) RETURNING id, title, type_id, owner_id", req.Title, req.TypeId, reqAcc.Id).Scan(&act.Id, &act.Title, &act.TypeId, &act.OwnerId)
	if err != nil {
		return nil, err
	}
	_, err = srvr.db.Exec("INSERT INTO participants (account_id, activity_id) VALUES ($1, $2)", reqAcc.Id, act.Id)
	if err != nil {
		return nil, err
	}
	return &act, nil
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
	case "DELETE":
		err := deleteActivityById(w, r)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
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

func deleteActivityById(w http.ResponseWriter, r *http.Request) error {
	params := params(r)
	_, err := srvr.db.Exec("DELETE FROM activities WHERE id = $1", params["activityId"])
	if err != nil {
		return err
	}
	return nil
}

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		a, err := getAccounts(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(a)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func getAccounts(w http.ResponseWriter, r *http.Request) ([]account, error) {
	rows, err := srvr.db.Query("SELECT id, name, email FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accs []account
	for rows.Next() {
		var acc account
		if err := rows.Scan(&acc.Id, &acc.Name, &acc.Email); err != nil {
			return nil, err
		}
		accs = append(accs, acc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accs, nil
}

func handleAccountsById(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		a, err := getAccountById(w, r)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := json.Marshal(a)
		if err != nil {
			fmt.Println(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func getAccountById(w http.ResponseWriter, r *http.Request) (*account, error) {
	params := params(r)
	var acc account
	err := srvr.db.QueryRow("SELECT id, name, email FROM accounts WHERE id = $1", params["accountId"]).Scan(&acc.Id, &acc.Name, &acc.Email)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

func encryptPwd(p string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func comparePwd(p string, ep string) error {
	err := bcrypt.CompareHashAndPassword([]byte(ep), []byte(p))
	if err != nil {
		return err
	}
	return nil
}

type claims struct {
	Email string `json:"email"`
	jwt.StandardClaims
}

func generateAccessToken(email string) (string, error) {
	c := claims{
		Email: email,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
			Issuer:    "ja",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	tokenString, err := token.SignedString(signingKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func validateAccessToken(ts string) (*claims, error) {
	token, err := jwt.ParseWithClaims(ts, &claims{}, func(t *jwt.Token) (interface{}, error) {
		return signingKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid access token")
	}
	return claims, nil
}
