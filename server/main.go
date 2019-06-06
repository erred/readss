package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	grpcweb "github.com/seankhliao/go-grpcweb"
	"google.golang.org/grpc"

	pb "github.com/seankhliao/readss/readss"
)

func main() {
	svr := NewServer("/etc/readss/subs.csv", 30*time.Minute)
	gsvr := grpc.NewServer()

	// register svr with gsvr
	pb.RegisterListerServer(gsvr, svr)

	// wrap grpc handler in grpc-web handler
	handler := grpcweb.New(gsvr)

	// OPTIONAL:
	// handle cors if necessary:
	//  Headers:
	//    Access-Control-Allow-Origin
	//    Access-Control-Allow-Headers
	//  Request:
	//    method: OPTIONS
	//    response: 200
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		defer fmt.Println("request took ", time.Now().Sub(t).Nanoseconds(), "ns")
		w.Header().Set("Access-Control-Expose-Headers", "grpc-status, grpc-message")
		w.Header().Set("Access-Control-Allow-Headers", "origin, content-type, x-grpc-web, x-user-agent")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST")
		// only a single origin is allowed
		w.Header().Set("Access-Control-Allow-Origin", "https://seankhliao.com")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler.ServeHTTP(w, r)
	})
	http.ListenAndServe(":8090", h)

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
					Source:  sub.Name,
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
