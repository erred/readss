package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/mmcdole/gofeed"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	f := flag.String("f", "subs.xml", "OPML file of RSS subscriptions to parse")
	p := flag.String("p", "8080", "port to listen on")
	t := flag.String("t", "template.html", "html template to use")
	tz := flag.String("tz", "Asia/Taipei", "timezone for reltime")
	u := flag.Int64("u", 30, "update interval, minutes")
	flag.Parse()

	sub, err := NewSub(*f, *t, *tz)
	if err != nil {
		log.Fatal(err)
	}

	go sub.tick(time.Duration(*u) * time.Minute)

	http.HandleFunc("/", sub.handler)
	http.ListenAndServe(":"+*p, nil)
}

type Sub struct {
	opml string
	temp string
	tz   string
	ol   []OPMLOutline
	tmpl *template.Template
	loc  *time.Location
	feed Feed
	buf  *bytes.Buffer
	etag string
}

func NewSub(f, t, tz string) (*Sub, error) {
	sub := &Sub{
		opml: f,
		temp: t,
		tz:   tz,
		buf:  &bytes.Buffer{},
	}
	err := sub.parseOPML()
	if err != nil {
		return sub, err
	}
	err = sub.parseTemplate()
	if err != nil {
		return sub, err
	}
	err = sub.parseLocation()
	if err != nil {
		return sub, err
	}
	sub.feed = Feed{
		Updated: humanTime(time.Now(), sub.loc),
		Errors:  []error{fmt.Errorf("Please wait for initial load")},
		NextUp:  time.Now(),
	}
	err = sub.tmpl.Execute(sub.buf, sub.feed)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func (s *Sub) parseOPML() error {
	opml, err := NewOPMLFile(s.opml)
	if err != nil {
		return fmt.Errorf("parse OPML %s: %s", s.opml, err)
	}
	s.ol = opml.Body.Outlines
	return nil
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

func (s *Sub) parseTemplate() error {
	tmpl, err := template.ParseFiles(s.temp)
	if err != nil {
		return fmt.Errorf("parse template: %s: %s", s.temp, err)
	}
	s.tmpl = tmpl
	return nil
}
func (s *Sub) parseLocation() error {
	loc, err := time.LoadLocation(s.tz)
	if err != nil {
		return fmt.Errorf("load location %s: %s", s.tz, err)
	}
	s.loc = loc
	return nil
}

func (s *Sub) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Header.Get("ETag") == s.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if d := s.feed.NextUp.Sub(time.Now()); d <= 0 {
		w.Header().Add("Cache-Control", "no-cache")
	} else {
		w.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d", int(d.Seconds())))
	}

	_, err := io.Copy(w, s.buf)
	if err != nil {
		log.Println("copy template: ", err)
	}
}

func (s *Sub) tick(d time.Duration) {
	s.feed = s.getAll(d)

	t := time.NewTicker(d)
	for range t.C {
		err := s.parseOPML()
		if err != nil {
			s.feed = Feed{
				Updated: humanTime(time.Now(), s.loc),
				Errors:  []error{fmt.Errorf("Error parsing OPML: %v", err)},
			}
			continue
		}
		err = s.parseTemplate()
		if err != nil {
			s.feed = Feed{
				Updated: humanTime(time.Now(), s.loc),
				Errors:  []error{fmt.Errorf("Error parsing html template: %v", err)},
			}
			continue
		}
		s.feed = s.getAll(d)
	}
}

func (s *Sub) getAll(d time.Duration) Feed {
	log.Println("updating feed")
	feed := Feed{
		Updated:  humanTime(time.Now(), s.loc),
		Interval: int(d / time.Minute),
		NextUp:   time.Now().Add(d),
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
	feed.limit()

	s.tmpl.Execute(s.buf, s.feed)
	et := make([]byte, 8)
	rand.Read(et)
	s.etag = base64.StdEncoding.EncodeToString(et)
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
	if ago == "0m ago" {
		ago = ""
	}
	return t.In(loc).Format("2006-01-02 15:04 ") + ago
}

type Feed struct {
	Items    []Item
	Errors   []error
	Updated  string
	NextUp   time.Time
	Interval int
}

func (f Feed) Sort() { sort.Sort(Items(f.Items)) }

// limit output to 6 months
func (f Feed) limit() {
	for i, v := range f.Items {
		if time.Now().Sub(v.Timestamp) >= 6*30*24*time.Hour {
			f.Items = f.Items[:i]
			return
		}
	}
}

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
