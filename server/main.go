package main

import (
	"context"
	"encoding/csv"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/mmcdole/gofeed"
	"google.golang.org/grpc"

	"github.com/seankhliao/readss/readss"
)

var (
	// grpc stuff
	Debug   = false
	Headers = strings.Split(os.Getenv("HEADERS"), ",")
	Origins = make(map[string]struct{})
	Port    = os.Getenv("PORT")

	// service stuff
	Config = os.Getenv("CONFIG")
	Tick   = 30 * time.Minute
)

func init() {
	// grpc stuff
	if os.Getenv("DEBUG") == "1" {
		Debug = true
	}

	for i, h := range Headers {
		Headers[i] = strings.TrimSpace(h)
	}

	for _, o := range strings.Split(os.Getenv("ORIGINS"), ",") {
		Origins[strings.TrimSpace(o)] = struct{}{}
	}

	if Port == "" {
		Port = ":8090"
	}
	if Port[0] != ':' {
		Port = ":" + Port
	}

	// service stuff
	if Config == "" {
		Config = "/etc/readss/subs.csv"
	}

	if d, err := time.ParseDuration(os.Getenv("TICK")); err == nil {
		Tick = d
	}
}

func allowOrigin(o string) bool {
	_, ok := Origins[o]
	if Debug {
		log.Printf("origin filter %v allowed: %v\n", o, ok)
	}
	return ok
}

func main() {
	svr := NewServer(Config, Tick)
	gsvr := grpc.NewServer()
	readss.RegisterListerServer(gsvr, svr)
	wsvr := grpcweb.WrapServer(gsvr,
		grpcweb.WithOriginFunc(allowOrigin),
		grpcweb.WithAllowedRequestHeaders(Headers),
	)

	if Debug {
		log.Printf("read config at %v, ticking at %v\n", Config, Tick)
		log.Printf("starting on %v\nallowing headers: %v\nallowing origins: %v\n",
			Port, Headers, Origins)
	}
	http.ListenAndServe(Port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		wsvr.ServeHTTP(w, r)
	}))
}

type Server struct {
	ats  []*readss.Article
	fn   string
	tick time.Duration
}

func NewServer(fn string, tick time.Duration) *Server {
	svr := &Server{
		fn:   fn,
		tick: tick,
	}
	go svr.updater()
	return svr
}

func (s *Server) List(context.Context, *readss.ListRequest) (*readss.ListReply, error) {
	return &readss.ListReply{
		Articles: s.ats,
	}, nil
}

func (s *Server) updater() {
	s.ats = getArticles(parseSubs(s.fn))
	for range time.NewTicker(s.tick).C {
		s.ats = getArticles(parseSubs(s.fn))
	}
}

type Sub struct {
	Name     string
	URL      string
	Articles []*readss.Article
}

func parseSubs(fn string) []Sub {
	f, err := os.Open(fn)
	if err != nil {
		log.Printf("parseSubs open %v: %v\n", fn, err)
		return nil
	}
	defer f.Close()

	rr, err := csv.NewReader(f).ReadAll()
	if err != nil {
		log.Printf("parseSubs readall %v\n", err)
		return nil
	}

	subs := make([]Sub, len(rr))
	for i := range subs {
		subs[i] = Sub{
			Name: rr[i][0],
			URL:  rr[i][1],
		}
	}
	return subs
}

func getArticles(subs []Sub) []*readss.Article {
	if Debug {
		log.Printf("starting getArticles")
		defer log.Printf("finsihed getArticles")
	}
	var wg sync.WaitGroup
	for s, sub := range subs {
		wg.Add(1)
		go func(s int, sub Sub) {
			defer wg.Done()

			feed, err := gofeed.NewParser().ParseURL(sub.URL)
			if err != nil {
				log.Printf("getSubs get feed %v: %v\n", sub.Name, err)
				return
			}
			ats := make([]*readss.Article, len(feed.Items))
			for i, it := range feed.Items {
				ts := it.PublishedParsed
				if it.UpdatedParsed != nil {
					ts = it.UpdatedParsed
				}
				ats[i] = &readss.Article{
					Title:   it.Title,
					Url:     it.Link,
					Source:  sub.Name,
					Time:    ts.Format("2006-01-02 15:04"),
					Reltime: humanTime(*ts),
				}
			}

			subs[s].Articles = ats
		}(s, sub)
	}
	wg.Wait()

	var ats []*readss.Article
	for _, s := range subs {
		ats = append(ats, s.Articles...)
	}
	sort.Sort(Articles(ats))
	if len(ats) > 100 {
		ats = ats[:100]
	}
	return ats
}
func humanTime(t time.Time) string {
	d := time.Now().Sub(t)
	var ago string
	switch {
	case d > -time.Hour && d < time.Hour:
		ago = strconv.FormatInt(d.Nanoseconds()/int64(time.Minute), 10) + "m ago"
	case d > -24*time.Hour && d < 24*time.Hour:
		ago = strconv.FormatInt(d.Nanoseconds()/int64(time.Hour), 10) + "h ago"
	default:
		ago = strconv.FormatInt(d.Nanoseconds()/int64(24*time.Hour), 10) + "d ago"
	}
	if ago == "0m ago" {
		ago = ""
	}
	return ago
}

type Articles []*readss.Article

func (a Articles) Len() int           { return len(a) }
func (a Articles) Less(i, j int) bool { return a[i].Time > a[j].Time }
func (a Articles) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
