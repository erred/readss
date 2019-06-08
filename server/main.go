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

	pb "github.com/seankhliao/readss/readss"
)

var (
	Config  = os.Getenv("CONFIG")
	Debug   bool
	Headers []string
	Origins map[string]struct{}
	Port    = os.Getenv("PORT")
	Tick    time.Duration
)

func init() {
	if Port == "" {
		Port = ":8090"
	} else if Port[0] != ':' {
		Port = ":" + Port
	}
	if os.Getenv("DEBUG") == "1" {
		Debug = true
	}

	Origins = make(map[string]struct{})
	for _, o := range strings.Split(os.Getenv("ORIGINS"), ",") {
		Origins[strings.TrimSpace(o)] = struct{}{}
	}

	if Config == "" {
		Config = "/etc/readss/subs.csv"
	}

	if d, err := time.ParseDuration(os.Getenv("TICK")); err != nil {
		Tick = 30 * time.Minute
	} else {
		Tick = d
	}
	Headers = strings.Split(os.Getenv("HEADERS"), ",")
	for i, h := range Headers {
		Headers[i] = strings.TrimSpace(h)
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
	pb.RegisterListerServer(gsvr, svr)
	wsvr := grpcweb.WrapServer(gsvr,
		grpcweb.WithOriginFunc(allowOrigin),
		grpcweb.WithAllowedRequestHeaders(Headers),
	)

	if Debug {
		log.Printf("read config at %v, ticking at %v\n", Config, Tick)
		log.Printf("starting on %v allowing origins %v\n", Port, Origins)
	}
	http.ListenAndServe(Port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		w.Header().Add("Access-Control-Expose-Headers", "grpc-status")
		w.Header().Add("Access-Control-Expose-Headers", "grpc-message")
		wsvr.ServeHTTP(w, r)
	}))
}

type Server struct {
	ats  []*pb.Article
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

func (s *Server) List(context.Context, *pb.ListRequest) (*pb.ListReply, error) {
	return &pb.ListReply{
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
	Articles []*pb.Article
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

func getArticles(subs []Sub) []*pb.Article {
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
			ats := make([]*pb.Article, len(feed.Items))
			for i, it := range feed.Items {
				ts := it.PublishedParsed
				if it.UpdatedParsed != nil {
					ts = it.UpdatedParsed
				}
				ats[i] = &pb.Article{
					Title:   it.Title,
					Url:     it.Link,
					Source:  feed.Title,
					Time:    ts.Format("2006-01-02 15:04"),
					Reltime: humanTime(*ts),
				}
			}

			subs[s].Articles = ats
		}(s, sub)
	}
	wg.Wait()

	var ats []*pb.Article
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

type Articles []*pb.Article

func (a Articles) Len() int           { return len(a) }
func (a Articles) Less(i, j int) bool { return a[i].Time > a[j].Time }
func (a Articles) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
