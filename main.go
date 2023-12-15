package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"memo/pkg/database"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	godotenv.Load()
	port := os.Getenv("PORT")
	if len(os.Args) > 1 {
		port = os.Args[1]
		if port == "ssl" {
			port = "443"
		}
	}
	if port == "" {
		port = "5000"
	}

	mux := http.NewServeMux()
	mux.Handle("/st/", http.StripPrefix("/st/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/", IndexHandle)
	log.Println("Listening on port: " + port)
	if port == "443" {
		log.Println("SSL")
		if err := http.Serve(autocert.NewListener(os.Getenv("DOMAIN")), mux); err != nil {
			panic(err)
		}
	} else if err := http.ListenAndServe(":"+port, mux); err != nil {
		panic(err)
	}
}

func IndexHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")

	if r.Method == http.MethodGet {
		if r.URL.Path[1:] != "" {
			tag := r.URL.Path[1:]
			db := database.Connect()
			defer db.Close()
			txt := ""
			q := "select txt from memo where tag = '" + database.Escape(tag) + "' order by p"
			rows, err := db.Query(q)
			if err != nil {
				log.Println(err)
				http.Error(w, "error 0", 500)
				return
			}
			defer rows.Close()
			exist := false
			for rows.Next() {
				exist = true
				var str string
				err = rows.Scan(&str)
				if err != nil {
					log.Println(err)
					http.Error(w, "error 1", 500)
					return
				}
				txt += str
			}
			if exist {
				w.Header().Add("Content-Disposition", "attachment")
				fmt.Fprint(w, txt)
			} else {
				http.Error(w, "not found", 404)
			}
			return
		}
		http.Error(w, "content nothing", 404)
		return
	} else if r.Method == http.MethodPost {
		if r.FormValue("text") != "" && r.FormValue("tag") != "" {
			db := database.Connect()
			defer db.Close()
			tran, err := db.Begin()
			if err != nil {
				log.Println(err)
				http.Error(w, "error 0", 500)
				return
			}
			tag := strings.TrimSpace(r.FormValue("tag"))
			del, err := db.Query("delete from memo where tag = '" + database.Escape(tag) + "'")
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), 500)
				return
			}
			del.Close()
			txt := r.FormValue("text")
			page := 0
			for len([]rune(txt)) > 0 {
				str := []rune(txt)
				if len(str) > 10000 {
					txt = string(str[10000:])
					str = str[:10000]
				} else {
					txt = ""
				}
				q := "insert into memo (tag, p, txt) values (?, ?, ?)"
				ins, err := tran.Prepare(q)
				if err != nil {
					log.Println(err)
					http.Error(w, "error 1", 500)
					tran.Rollback()
					return
				}
				_, err = ins.Exec(tag, page, string(str))
				ins.Close()
				if err != nil {
					log.Println(err)
					http.Error(w, "error 2", 500)
					tran.Rollback()
					return
				}
				page++
			}
			tran.Commit()
			fmt.Fprint(w, "Complete insert")
			return
		} else {
			http.Error(w, "not found", 404)
		}
	}
}
