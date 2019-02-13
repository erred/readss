package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/mmcdole/gofeed"
)

func main() {
	f := flag.String("f", "subs.xml", "OPML file of RSS subscriptions to parse")
	p := flag.String("p", "8080", "port to listen on")
	t := flag.String("t", "template.html", "html template to use")
	u := flag.Int64("u", 30, "update interval, minutes")
	flag.Parse()

	sub, err := NewSub(*f, *t)
	if err != nil {
		log.Fatal(err)
	}
	go sub.tick(time.Duration(*u) * time.Minute)
	http.HandleFunc("/", sub.handler)
	http.ListenAndServe(":"+*p, nil)
}

type Sub struct {
	ol   []OPMLOutline
	tmpl *template.Template
	loc  *time.Location
	feed Feed
}

func NewSub(f, t string) (*Sub, error) {
	opml, err := NewOPMLFile(f)
	if err != nil {
		return nil, fmt.Errorf("parse OPML %s: %s", f, err)
	}
	tmpl, err := template.ParseFiles(t)
	if err != nil {
		return nil, fmt.Errorf("parse template: %s: %s", t, err)
	}

	l := "Asia/Taipei"
	loc, err := time.LoadLocation(l)
	if err != nil {
		return nil, fmt.Errorf("load location %s: %s", l, err)
	}

	return &Sub{
		ol:   opml.Body.Outlines,
		tmpl: tmpl,
		loc:  loc,
		feed: Feed{
			Updated: humanTime(time.Now(), loc),
			Errors:  []error{fmt.Errorf("Please wait for initial load")},
		},
	}, nil
}

func (s *Sub) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	err := s.tmpl.Execute(w, s.feed)
	if err != nil {
		log.Println("exec template: ", err)
	}
}

func (s *Sub) tick(d time.Duration) {
	s.feed = s.getAll()

	t := time.NewTicker(d)
	for range t.C {
		s.feed = s.getAll()
	}
}

func (s *Sub) getAll() Feed {
	log.Println("updating feed")
	feed := Feed{
		Updated: humanTime(time.Now(), s.loc),
	}
	c := make(chan Iterr, 100)
	for _, o := range s.ol {
		go getFeed(o.Title, o.XMLURL, c)
	}

	done := len(s.ol)
	for i := range c {
		if i.d || i.e != nil {
			done--
			if i.e != nil {
				feed.Errors = append(feed.Errors, i.e)
			}
			if done == 0 {
				break
			}
		} else if i.e != nil {
			done--
		} else {
			i.i.Updated = humanTime(i.i.Timestamp, s.loc)
			feed.Items = append(feed.Items, i.i)
		}
	}
	close(c)

	feed.Sort()
	if len(feed.Errors) > 0 {
		log.Println("Errors getting feed:")
		for _, err := range feed.Errors {
			log.Println(err)
		}
	}
	log.Println("updated feed")
	return feed
}

func getFeed(title, url string, c chan Iterr) {
	// fmt.Println("Getting feed: ", title)
	fp := gofeed.NewParser()
	f, err := fp.ParseURL(url)
	if err != nil {
		c <- Iterr{e: fmt.Errorf("feed %s err: %s", title, err)}
		return
	}
	// fmt.Println("Got feed: ", title, " items: ", len(f.Items))
	for _, i := range f.Items {
		ts := i.PublishedParsed
		if i.UpdatedParsed != nil {
			ts = i.UpdatedParsed
		}
		c <- Iterr{i: Item{
			Source:    f.Title,
			Link:      template.URL(i.Link),
			Title:     i.Title,
			Timestamp: *ts,
		}}
	}
	// fmt.Println("Got feed: ", title, " done")
	c <- Iterr{d: true}
}

func humanTime(t time.Time, loc *time.Location) string {
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
	return t.In(loc).Format("2006-01-02 15:04 ") + ago
}

type Feed struct {
	Items   []Item
	Errors  []error
	Updated string
}

func (f Feed) Sort() { sort.Sort(Items(f.Items)) }

type Items []Item

func (s Items) Add(i Item) { s = append(s, i) }
func (s Items) Len() int   { return len(s) }

// force Descending Now -> Older
func (s Items) Less(i, j int) bool { return s[i].Timestamp.After(s[j].Timestamp) }
func (s Items) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type Iterr struct {
	i Item
	e error
	d bool
}

type Item struct {
	Source    string
	Link      template.URL
	Title     string
	Timestamp time.Time
	Updated   string
}

func NewOPMLFile(fn string) (*OPML, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return NewOPML(f)
}
func NewOPML(r io.Reader) (*OPML, error) {
	var o = &OPML{}
	return o, xml.NewDecoder(r).Decode(o)
}

type OPML struct {
	Body struct {
		Outlines []OPMLOutline `xml:"outline"`
	} `xml:"body"`
}
type OPMLOutline struct {
	Text    string `xml:"text,attr"`
	Title   string `xml:"title,attr"`
	Type    string `xml:"type,attr"`
	XMLURL  string `xml:"xmlUrl,attr"`
	HTMLURL string `xml:"htmlUrl,attr"`
}