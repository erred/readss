# ![logo](static/icon-64.png) ReadSS

Server side RSS link aggregator

[![License](https://img.shields.io/github/license/seankhliao/readss.svg?style=for-the-badge&maxAge=31536000)](LICENSE)
[![Build](https://badger.seankhliao.com/i/github_seankhliao_readss)](https://badger.seankhliao.com/l/github_seankhliao_readss)

## About

I just wanted a constantly updated list of links to blog posts

Re-reads OPML / template every update, (live update)

SW / PWA for nice add to homescreen (not really offline)

## Usage

#### Prerequisites

- docker

#### Run

docker:

```sh
make run
```

```sh
readss [-p 8080] [-f subs.xml] [-t template.html] [-u 30] [-tz Asia/Taipei]
    -p  port
    -f  OPML subscription xml file
    -t  Go html/template to render, passes in a Feed
    -tz timezone
    -u  update interval, minutes
```

#### Build

```sh
make build
```

## Todo

- [ ] icon / logo
- [ ] add to homescreen
- [ ] add compression
- [ ] force refresh
- [x] consistent branding: ReadSS
- [x] pregen / cache result
- [x] limit nodes
- [x] cache control headers
- [x] fix time ago
- [x] fix mobile spacing
- [x] ~~offline~~
  - just links, you wouldn't be able to read anything anyways
