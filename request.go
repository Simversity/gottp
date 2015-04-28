package gottp

import (
	"compress/gzip"
	"compress/zlib"
	"log"
	"net/http"
	"strconv"
	"strings"

	"gopkg.in/simversity/gottp.v2/utils"
)

//*http.Request, rw ResponseWriter

const PAGE_SIZE = 30
const SKIP = 0

type WireSender interface {
	SendOverWire() utils.Q
}

type Paginator struct {
	Skip  int
	Limit int
	Wlt   string
	Wgt   string
	Wkey  string
	Ids   []string
}

func makeInt(val interface{}, fallback int) int {
	switch val.(type) {
	case int:
		return val.(int)
	case string:
		_val := val.(string)
		ret, err := strconv.Atoi(_val)
		if err == nil {
			return ret
		}
	case float64:
		return int(val.(float64))
	}
	return fallback
}

func makeString(val interface{}) (ret string) {
	switch val.(type) {
	case string:
		ret = val.(string)
	case int:
		val := val.(int)
		ret = strconv.Itoa(val)
	}
	return
}

type Request struct {
	Request    *http.Request
	Writer     http.ResponseWriter
	UrlArgs    *map[string]string
	pipeOutput chan<- *utils.Q
	pipeIndex  int
	params     *utils.Q
}

func (r *Request) makeUrlArgs() {
	original := *r.params
	if r.UrlArgs != nil {
		for key, value := range *r.UrlArgs {
			original[key] = value
		}
	}
}

func (r *Request) GetPaginator() *Paginator {
	p := Paginator{Limit: -1}
	qp := r.GetArguments()
	for key, value := range *qp {
		switch key {
		case "skip":
			p.Skip = makeInt(value, SKIP)
		case "limit":
			p.Limit = makeInt(value, PAGE_SIZE)
		case "wlt":
			p.Wlt = makeString(value)
		case "wgt":
			p.Wgt = makeString(value)
		case "wkey":
			p.Wkey = makeString(value)
		case "ids":
			ids, ok := value.([]string)
			if !ok {
				id := value.(string)
				p.Ids = []string{id}
			} else {
				p.Ids = ids
			}
		}
	}

	if p.Limit < 0 {
		p.Limit = PAGE_SIZE
	}

	return &p
}

func (r *Request) makeUrlParams() {
	original := *r.params
	r.Request.ParseForm()
	for key, value := range r.Request.Form {
		length := len(value)
		if length == 1 {
			original[key] = value[0]
		} else {
			original[key] = value
		}
	}
}

func (r *Request) makeBodyParams() {
	//if (r.Request.Method == "PUT" || r.Request.Method == "POST") {
	if r.Request.ContentLength != 0 {
		utils.DecodeStream(r.Request.Body, r.params)
	}
}

func (r *Request) GetArguments() *utils.Q {
	if r.params == nil {
		r.params = &utils.Q{}
		r.makeUrlArgs()
		r.makeBodyParams()
		r.makeUrlParams()
	}

	return r.params
}

func (r *Request) ConvertArguments(f interface{}) {
	utils.Convert(r.GetArguments(), f)
}

func (r *Request) GetArgument(key string) interface{} {
	args := *r.GetArguments()
	return args[key]
}

func (r *Request) ConvertArgument(key string, f interface{}) {
	args := *r.GetArguments()
	val := args[key]
	utils.Convert(val, f)
}

func (r *Request) Finish(data interface{}) []byte {
	r.Writer.Header().Set("Server", serverUA)
	r.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	r.Writer.Header().Set("Content-Type", "application/json")
	return utils.Encoder(data)
}

func (r *Request) Redirect(url string, status int) {
	log.Println("Redirecting to", url)
	http.Redirect(r.Writer, r.Request, url, status)
	return
}

func (r *Request) Write(data interface{}) {
	var piped utils.Q

	if v, ok := data.(WireSender); ok {
		piped = v.SendOverWire()
	} else {
		piped = utils.Q{
			"data":    data,
			"status":  http.StatusOK,
			"message": "",
		}
	}

	if r.pipeOutput != nil {
		piped["index"] = r.pipeIndex
		r.pipeOutput <- &piped
	} else if strings.Contains(
		r.Request.Header.Get("Accept-Encoding"), "deflate",
	) {
		r.Writer.Header().Set("Content-Encoding", "deflate")

		gz := zlib.NewWriter(r.Writer)
		defer gz.Close()
		gz.Write(r.Finish(piped))

	} else if strings.Contains(
		r.Request.Header.Get("Accept-Encoding"), "gzip",
	) {
		r.Writer.Header().Set("Content-Encoding", "gzip")

		gz := gzip.NewWriter(r.Writer)
		defer gz.Close()
		gz.Write(r.Finish(piped))

	} else {
		r.Writer.Write(r.Finish(piped))
	}
}

func (r *Request) Raise(e HttpError) {
	r.Write(e)
}
