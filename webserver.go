package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	db *sql.DB
}

func (app *App) logHandler(f func(
	w http.ResponseWriter,
	r *http.Request)) func(
	w http.ResponseWriter,
	r *http.Request) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		headers := ""
		for k, v := range r.Header {
			headers += fmt.Sprintf("%q: %q\n", k, v[0])
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		parts := strings.Split(r.RequestURI, "?")

		path := parts[0]
		query := ""
		if len(parts) > 1 {
			query = parts[1]
		}

		_, errInsert := app.db.Exec(`INSERT INTO log
            (method, path, query, headers, body, created)
            VALUES ($1, $2, $3, $4, $5, $6)`,
			r.Method, path, query, headers, string(body), time.Now())
		if errInsert != nil {
			http.Error(w, errInsert.Error(), http.StatusInternalServerError)
			return
		}

		f(w, r)
	}

	return handler
}

func (app *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "login")
}

func (app *App) checkHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "check")
}

func (app *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "logout")
}

func (app *App) statsHandler(w http.ResponseWriter, r *http.Request) {
	type Record struct {
		Path    string
		Counter int
	}

	type Response struct {
		Data []Record
	}

	rows, err := app.db.Query(`SELECT path, COUNT(id) counter FROM log GROUP BY path`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	js := Response{
		Data: []Record{},
	}

	for rows.Next() {
		record := Record{}
		rows.Scan(&record.Path, &record.Counter)
		js.Data = append(js.Data, record)
	}

	result, err := json.Marshal(js)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, string(result))
}

func (app *App) statsDetailHandler(w http.ResponseWriter, r *http.Request) {
	type Record struct {
		Method  string
		Query   string
		Created string
	}

	type Response struct {
		Data []Record
	}

	pathes := r.URL.Query()["path"]
	if len(pathes[0]) < 1 {
		http.Error(w, "Url Param 'path' is missing", http.StatusInternalServerError)
		return
	}

	path := pathes[0]

	rows, err := app.db.Query(`SELECT method, query, created FROM log WHERE path = $1 ORDER BY id`, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	js := Response{
		Data: []Record{},
	}

	for rows.Next() {
		record := Record{}
		rows.Scan(&record.Method, &record.Query, &record.Created)
		js.Data = append(js.Data, record)
	}

	result, err := json.Marshal(js)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, string(result))
}

func main() {

	db, err := sql.Open("sqlite3", "file:data.sqlite")
	if err != nil {
		panic(err)
	}

	app := &App{db: db}

	db.Exec(`
        CREATE TABLE IF NOT EXISTS log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            method text NOT NULL,
            path text NOT NULL,
            query text NOT NULL,
            headers text NOT NULL,
            body text NOT NULL,
            created timestring
        );
    `)

	http.HandleFunc("/login/", app.logHandler(app.loginHandler))
	http.HandleFunc("/check/", app.logHandler(app.checkHandler))
	http.HandleFunc("/logout/", app.logHandler(app.logoutHandler))

	http.HandleFunc("/stats/", app.statsHandler)
	http.HandleFunc("/stats/detail/", app.statsDetailHandler)

	http.ListenAndServe(":8000", nil)
}
