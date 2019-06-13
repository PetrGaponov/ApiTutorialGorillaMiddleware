// app.go
//https://github.com/gorilla/handlers
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/Sirupsen/logrus"
	//log "github.com/sirupsen/logrus"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type App struct {
	Router *mux.Router
	DB     *sql.DB
	Logger *logrus.Logger
}

//

func parseRecoveryOptions(h http.Handler, opts ...RecoveryOption) http.Handler {

	for _, option := range opts {
		option(h)
	}

	return h
}

type RecoveryOption func(http.Handler)

type RecoveryHandlerLogger interface {
	Println(...interface{})
}

type recoveryHandler struct {
	handler    http.Handler
	logger     RecoveryHandlerLogger
	printStack bool
}

func use(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	// for _, m := range middleware {
	// 	h = m(h)
	// }
	for i := len(middleware) - 1; i >= 0; i-- { //reverse  for apply in right way
		h = middleware[i](h)

	}
	return h
}

//respondWithError(w, http.StatusInternalServerError, err.Error())

func RecoveryHandler(opts ...RecoveryOption) func(h http.Handler) http.Handler {

	return func(h http.Handler) http.Handler {
		r := &recoveryHandler{handler: h}
		return parseRecoveryOptions(r, opts...)
	}
}

// RecoveryLogger is a functional option to override
// the default logger
func RecoveryLogger(logger RecoveryHandlerLogger) RecoveryOption {
	return func(h http.Handler) {
		r := h.(*recoveryHandler)
		r.logger = logger
	}
}

func (h recoveryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			//w.WriteHeader(500)
			w.Write([]byte("Internal server error"))
			h.log(err)
		}
	}()

	h.handler.ServeHTTP(w, req)
}

func (h recoveryHandler) log(v ...interface{}) {
	if h.logger != nil {
		h.logger.Println(v...)
	} else {
		log.Println(v...)
	}

	if h.printStack {
		debug.PrintStack()
	}
}

//
func (a *App) LogMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//a.Logger.SetOutput(os.Stdout) // logs go to Stderr by default
		a.Logger.Println(r.Method, r.URL)
		h.ServeHTTP(w, r) // call ServeHTTP on the original handler

	})
}

func (a *App) LogMiddleware2(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//a.Logger.SetOutput(os.Stdout) // logs go to Stderr by default
		a.Logger.Println("logger2")
		a.Logger.Println(r.Method, r.URL)
		h.ServeHTTP(w, r) // call ServeHTTP on the original handler

	})
}

//
func (a *App) LogMiddleware3(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//a.Logger.SetOutput(os.Stdout) // logs go to Stderr by default
		a.Logger.Println("logger3")
		a.Logger.Println(r.Method, r.URL)
		panic("MyPanic")
		h.ServeHTTP(w, r) // call ServeHTTP on the original handler

	})
}

//

func (a *App) Initialize(user, password, dbname string) {
	connectionString :=
		fmt.Sprintf("user=%s password=%s dbname=%s", user, password, dbname)

	var err error
	a.DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}
	a.Router = mux.NewRouter()
	a.initializeRoutes()
	a.Logger = logrus.New()
	a.Logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(addr, RecoveryHandler()(a.Router)))
}

//
func MyCustom404Handler(w http.ResponseWriter, r *http.Request) {

	respondWithError(w, http.StatusInternalServerError, "Not found  Error")

}

//
func (a *App) initializeRoutes() {

	//a.Router.HandleFunc("/users", a.LogMiddleware(a.getUsers)).Methods("GET")
	//Make  custome http error not found
	a.Router.NotFoundHandler = http.HandlerFunc(MyCustom404Handler)
	//Make  chain  with  middlware
	a.Router.Handle("/users", use(http.HandlerFunc(a.getUsers), a.LogMiddleware2, a.LogMiddleware3)).Methods("GET")

	// apiMiddleware := []func(http.Handler) http.Handler{logging, apiAuth, json}
	// api := a.Router.PathPrefix("/api").SubRouter()
	// api.Handle("/users", use(usersHandler, apiMiddleware...))

	a.Router.HandleFunc("/user", a.LogMiddleware(a.createUser)).Methods("POST")
	a.Router.Handle("/user/{id:[0-9]+}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(a.getUser))).Methods("GET")
	a.Router.HandleFunc("/user/{id:[0-9]+}", a.LogMiddleware(a.updateUser)).Methods("PUT")
	a.Router.HandleFunc("/user/{id:[0-9]+}", a.LogMiddleware(a.deleteUser)).Methods("DELETE")
}

func (a *App) getUsers(w http.ResponseWriter, r *http.Request) {
	count, _ := strconv.Atoi(r.FormValue("count"))
	start, _ := strconv.Atoi(r.FormValue("start"))

	if count > 10 || count < 1 {
		count = 10
	}
	if start < 0 {
		start = 0
	}

	products, err := getUsers(a.DB, start, count)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, products)
}

func (a *App) createUser(w http.ResponseWriter, r *http.Request) {
	var u user
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&u); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if err := u.createUser(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, u)
}

func (a *App) getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	u := user{ID: id}
	if err := u.getUser(a.DB); err != nil {
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "User not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondWithJSON(w, http.StatusOK, u)
}

func (a *App) updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var u user
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&u); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
		return
	}
	defer r.Body.Close()
	u.ID = id

	if err := u.updateUser(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, u)
}

func (a *App) deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid User ID")
		return
	}

	u := user{ID: id}
	if err := u.deleteUser(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
