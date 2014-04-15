package manager

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"engines/github.com.bmizerany.pat"
)

var templates = make(map[template.Template]*template.Template)

func init() {
	//_, _ := template.ParseFiles("tmpl/body.html", "tmpl/header.html", "tmpl/footer.html")
}

type HttpManagement struct {
	Managed *[]Manager
}

func NewHttpManagement(managed *[]Manager) *HttpManagement {
	h := new(HttpManagement)
	h.Managed = managed
	return h
}

func (h *HttpManagement) SetRoutes(pat *pat.PatternServeMux) {
	pat.Get("/", http.HandlerFunc(h.HttpHandler))
	pat.Post("/", http.HandlerFunc(h.HttpHandler))
}

func (h *HttpManagement) HttpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		h.handleGet(w, r)
	} else if r.Method == "POST" {
		h.handlePost(w, r)
	}
}

func (h *HttpManagement) handlePost(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	idx, _ := strconv.ParseInt(r.FormValue("idx"), 10, 64)

	m := (*h.Managed)[idx]

	if action == "shutdown" && m.State().Up() {
		go Stop(m)
	} else if action == "startup" && m.State().Down() {
		go Start(m)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *HttpManagement) handleGet(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("tmpl/monitor.html", "tmpl/content.html", "tmpl/startup.html", "tmpl/shutdown.html")
	if err != nil {
		log.Fatal("Unable to open templates - make sure you are in GOPATH", err)
	}
	t.Execute(w, *h)
}
