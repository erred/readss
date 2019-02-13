# readss

[![License](https://img.shields.io/github/license/seankhliao/readss.svg?style=for-the-badge)](githib.com/seankhliao/readss)

Simple Server Side RSS reader

## usage

```
readss [-p 8080] [-f subs.xml] [-t template.html] [-u 30] [-tz Asia/Taipei]
    -p  port
    -f  OPML subscription xml file
    -t  Go html/template to render, passes in a Feed
    -tz timezone
    -u  update interval, minutes
```

re-reads opml / template every interval (useful if projecting config from k8s configMap)

## Ideas for improvement

- icon / logo
- add to homescreen
- add compression
- pregen / cache result
- limit nodes
- force refresh
- debug empty fields
- ~~offline~~
  - just links, you wouldn't be able to read anything anyways
