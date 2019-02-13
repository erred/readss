# readss

[![License](https://img.shields.io/github/license/seankhliao/readss.svg?style=for-the-badge)](githib.com/seankhliao/readss)

Simple Server Side RSS reader

## usage

```
readss [-p 8080] [-f subs.xml] [-t template.html] [-u 30]
    -p  port
    -f OPML subscription xml file
    -t Go html/template to render, passes in a Feed
    -u update interval, minutes
```
