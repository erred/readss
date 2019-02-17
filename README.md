# ReadSS

[![Build](https://img.shields.io/badge/endpoint.svg?url=https://badger.seankhliao.com/r/github_seankhliao_readss)](https://console.cloud.google.com/cloud-build/builds?project=com-seankhliao&query=source.repo_source.repo_name%20%3D%20%22github_seankhliao_readss%22)
[![License](https://img.shields.io/github/license/seankhliao/readss.svg?style=for-the-badge)](LICENSE)

Simple Server Side RSS reader

## Usage

```
readss [-p 8080] [-f subs.xml] [-t template.html] [-u 30] [-tz Asia/Taipei]
    -p  port
    -f  OPML subscription xml file
    -t  Go html/template to render, passes in a Feed
    -tz timezone
    -u  update interval, minutes
```

re-reads opml / template every interval (useful if projecting config from k8s configMap)

## TODO

- [ ] icon / logo
- [x] consistent branding: ReadSS
- [ ] add to homescreen
- [ ] add compression
- [x] pregen / cache result
- [x] limit nodes
- [ ] force refresh
- [ ] debug empty fields
- [x] cache control headers
- [x] fix time ago
- [x] fix mobile spacing
- [ ] ~~offline~~
  - just links, you wouldn't be able to read anything anyways
