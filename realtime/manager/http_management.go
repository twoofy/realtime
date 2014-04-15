package manager

import (
	"net/http"
	"html/template"
	//"log"

  "engines/github.com.bmizerany.pat"
)

var templates = make(map[template.Template]*template.Template)

func init() {
	//_, _ := template.ParseFiles("tmpl/body.html", "tmpl/header.html", "tmpl/footer.html")
	/*_, err := template.ParseFiles("tmpl/body.html", "tmpl/header.html", "tmpl/footer.html")
	if err != nil {
		log.Fatal("Unable to open templates ", err)
	}*/
}

type HttpManagement struct {
}

func NewHttpManagement() *HttpManagement {
	var h HttpManagement
	return &h
}



func (h *HttpManagement) SetRoutes(pat *pat.PatternServeMux) {
	pat.Get("/", http.HandlerFunc(h.HttpHandler))
}

func (h *HttpManagement) HttpHandler(w http.ResponseWriter, r *http.Request) {
}


